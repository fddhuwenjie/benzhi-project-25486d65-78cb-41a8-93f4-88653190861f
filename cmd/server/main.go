package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
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

const defaultListenAddress = "127.0.0.1:19081"

func configuredAddress() string {
	if port := os.Getenv("PORT"); port != "" {
		return "127.0.0.1:" + port
	}
	return defaultListenAddress
}

func main() {
	addr := flag.String("addr", configuredAddress(), "监听地址")
	selfCheck := flag.Bool("self-check", false, "执行有界自检")
	dataDir := flag.String("data", ".vitrinemon", "数据目录")
	flag.Parse()

	store, err := storage.New(filepath.Join(filepath.Clean(*dataDir), "snapshot.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "加载快照失败：", err)
		os.Exit(1)
	}
	log := events.New(filepath.Join(filepath.Clean(*dataDir), "events.jsonl"))
	app := inspection.New(store, log)
	server := &http.Server{
		Addr:              *addr,
		Handler:           web.New(app).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	if *selfCheck {
		if err := runSelfCheck(server, *addr); err != nil {
			fmt.Fprintln(os.Stderr, "自检失败：", err)
			os.Exit(1)
		}
		fmt.Println("自检通过：健康检查与巡检工作台可访问")
		return
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "HTTP 服务退出：", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func runSelfCheck(server *http.Server, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", addr, err)
	}
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(listener)
	}()

	client := &http.Client{Timeout: 2 * time.Second}
	for _, path := range []string{"/healthz", "/inspection"} {
		response, requestErr := client.Get("http://" + addr + path)
		if requestErr != nil {
			_ = server.Close()
			return fmt.Errorf("请求 %s: %w", path, requestErr)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			_ = server.Close()
			return fmt.Errorf("请求 %s 返回 HTTP %d", path, response.StatusCode)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return err
	}
	if err := <-serveDone; err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
