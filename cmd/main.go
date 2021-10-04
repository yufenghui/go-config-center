package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/go-homedir"
	"go-config-center/internal/service"
	"go-config-center/internal/store"
	"log"
	"net/http"
	"os"
	"time"
)

// Command line defaults
const (
	DefaultHTTPAddr  = "localhost:8080"
	DefaultRaftAddr  = "localhost:9000"
	DefaultRaftStore = "~/data/go-config-center"
)

// Command line parameters
var httpAddr string
var raftAddr string
var raftDataDir string
var joinAddr string
var nodeID string

func init() {
	flag.StringVar(&httpAddr, "haddr", DefaultHTTPAddr, "Set the HTTP bind address")
	flag.StringVar(&raftAddr, "raddr", DefaultRaftAddr, "Set Raft bind address")
	flag.StringVar(&raftDataDir, "rdir", DefaultRaftStore, "Set Raft data store path")
	flag.StringVar(&joinAddr, "join", "", "Set join address, if any")
	flag.StringVar(&nodeID, "id", "", "Node ID")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <raft-data-path> \n", raftDataDir)
		flag.PrintDefaults()
	}

	gin.SetMode(gin.ReleaseMode)
}

func main() {

	// 解析命令行参数
	flag.Parse()
	resolveParam()

	// 创建存储服务
	s := store.New()
	s.RaftAddr = raftAddr
	s.RaftDir = raftDataDir
	initRaft(s)

	// 创建并启动 service 服务
	h := service.New(httpAddr, s)
	h.Start()

}

func resolveParam() {
	raftDataDir, err := homedir.Expand(raftDataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Illegal Raft storage directory failed\n")
		os.Exit(1)
	}

	if err := os.MkdirAll(raftDataDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Create Raft storage directory failed\n")
		os.Exit(1)
	}
}

func initRaft(s *store.Store) error {

	err := s.Open(joinAddr == "", nodeID)
	if err != nil {
		log.Printf("failed to open store: %s", err.Error())
		return err
	}

	// if join was specified, make the join request
	if joinAddr != "" {
		err := join(joinAddr, httpAddr, raftAddr, nodeID)
		if err != nil {
			log.Printf("failed to join node at %s: %s", joinAddr, err.Error())
			return nil
		}
	} else {
		log.Println("no join addresses set")
	}

	// Wait until the store is in full consensus.
	openTimeout := 120 * time.Second
	s.WaitForLeader(openTimeout)
	s.WaitForApplied(openTimeout)

	// This may be a standalone server. In that case set its own metadata.
	if err := s.SetMeta(nodeID, httpAddr); err != nil && err != store.ErrNotLeader {
		// Non-leader errors are OK, since metadata will then be set through
		// consensus as a result of a join. All other errors indicate a problem.
		log.Printf("failed to SetMeta at %s: %s", nodeID, err.Error())
		return err
	}

	return nil
}

func join(joinAddr, httpAddr, raftAddr, nodeID string) error {
	b, err := json.Marshal(map[string]string{"httpAddr": httpAddr, "raftAddr": raftAddr, "id": nodeID})
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/join", joinAddr), "application-type/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
