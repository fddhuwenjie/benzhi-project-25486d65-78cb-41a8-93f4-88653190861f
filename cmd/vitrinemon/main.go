package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	"vitrinemon/internal/events"
	"vitrinemon/internal/inspection"
	"vitrinemon/internal/storage"
	"vitrinemon/internal/web"
)

func address() string {
	v := os.Getenv("PORT")
	if v != "" {
		return "127.0.0.1:" + v
	}
	return defaultListenAddress
}
func main() {
	addr := flag.String("addr", address(), "监听地址")
	self := flag.Bool("self-check", false, "执行有界自检")
	data := flag.String("data", ".vitrinemon", "数据目录")
	flag.Parse()
	_ = filepath.Clean(*data)
	store, err := storage.New(filepath.Join(*data, "snapshot.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "加载快照失败：", err)
		os.Exit(1)
	}
	log := events.New(filepath.Join(*data, "events.jsonl"))
	app := inspection.New(store, log)
	srv := &http.Server{Addr: *addr, Handler: web.New(app).Handler(), ReadHeaderTimeout: 5 * time.Second}
	if *self {
		runSelfCheck(srv, *addr)
		return
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
func runSelfCheck(srv *http.Server, addr string) {
	go srv.ListenAndServe()
	client := &http.Client{Timeout: 2 * time.Second}
	var err error
	for i := 0; i < 20; i++ {
		time.Sleep(25 * time.Millisecond)
		var resp *http.Response
		resp, err = client.Get("http://" + addr + "/healthz")
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 {
				break
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "自检健康检查失败：", err)
		srv.Shutdown(context.Background())
		os.Exit(1)
	}
	resp, err := client.Get("http://" + addr + "/inspection")
	if err != nil || resp.StatusCode != 200 {
		fmt.Fprintln(os.Stderr, "自检核心页面失败：", err)
		srv.Shutdown(context.Background())
		os.Exit(1)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	fmt.Println("自检通过：健康检查与巡检工作台可访问")
	srv.Shutdown(context.Background())
}
