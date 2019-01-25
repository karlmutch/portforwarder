package main

import (
	"context"
	"flag"
	"io"
	"net"
	"os"
	"os/signal"
	"path"
	"strconv"
	"time"

	logxi "github.com/karlmutch/logxi/v1"

	"github.com/karlmutch/errors" // Forked copy of https://github.com/jjeffery/errors
	"github.com/karlmutch/stack"  // Forked copy of https://github.com/go-stack/stack

	"github.com/karlmutch/envflag" // Forked copy of https://github.com/GoBike/envflag
)

var (
	target  = flag.String("target", "", "the target (<host>:<port>)")
	portOpt = flag.Int("port", 7757, "the incoming listening port")
	verbose = flag.Bool("v", false, "When enabled will print internal logging for this tool")
)

func main() {
	logger := logxi.New(path.Base(os.Args[0]))

	if !flag.Parsed() {
		envflag.Parse()
	}

	if *verbose {
		logger.SetLevel(logxi.LevelDebug)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 1)

	signal.Notify(signals, os.Interrupt)
	go func() {
		defer cancel()
		for _ = range signals {
			logger.Warn("stopping")
			return
		}
	}()

	port := strconv.Itoa(*portOpt)
	incoming, errGo := net.Listen("tcp", ":"+port)
	if errGo != nil {
		logger.Fatal(errors.Wrap(errGo, "could not start listening port").With("port", port, "stack", stack.Trace().TrimRuntime()).Error())
	}
	logger.Info("running", "port", port)

	go func() {
		logger := logxi.New("listener")
		for {
			conn, errGo := incoming.Accept()
			if errGo != nil {
				logger.Fatal(errors.Wrap(errGo, "accept failed").With("port", port, "stack", stack.Trace().TrimRuntime()).Error())
			}
			go func(client *net.TCPConn) {
				logger := logxi.New("handler")
				defer client.Close()
				logger.Info("connected", "address", client.RemoteAddr())
				defer logger.Info("closed", "address", client.RemoteAddr())

				server, errGo := net.Dial("tcp", *target)
				if errGo != nil {
					logger.Fatal(errors.Wrap(errGo, "target connection failed").With("address", target, "stack", stack.Trace().TrimRuntime()).Error())
				}
				defer server.Close()
				logger.Debug("established", "address", server.RemoteAddr())

				client.SetKeepAlive(true)
				client.SetKeepAlivePeriod(time.Second * 60)

				go func() {
					io.Copy(client, server)
				}()
				io.Copy(server, client)
			}(conn.(*net.TCPConn))
		}
	}()

	<-ctx.Done()
}
