package service

import (
	"encoding/json"
	"fmt"
	"github.com/fvbock/endless"
	"github.com/gin-gonic/gin"
	"go-config-center/internal/store"
	"net/http"
	"strings"
)

// Store is the interface Raft-backed key-value stores must implement.
type Store interface {
	// Get returns the value for the given key.
	Get(key string, lvl store.ConsistencyLevel) (string, error)

	// Set sets the value for the given key, via distributed consensus.
	Set(key, value string) error

	// Delete removes the given key, via distributed consensus.
	Delete(key string) error

	// Join joins the node, identitifed by nodeID and reachable at addr, to the cluster.
	Join(nodeID string, httpAddr string, addr string) error

	LeaderAPIAddr() string

	SetMeta(key, value string) error

	// Stats stats of the node
	Stats() interface{}
}

type Service struct {
	addr  string
	store *store.Store
}

func New(addr string, store *store.Store) *Service {
	return &Service{
		addr:  addr,
		store: store,
	}
}

func (s *Service) Start() error {
	router := gin.Default()
	router.GET("/ping", s.handlePing)
	router.GET("/stats", s.handleStats)
	router.GET("/data", s.handleStore)

	router.POST("/join", s.handleJoin)

	router.GET("/key/:key", s.handleKeyGet)
	router.DELETE("/key/:key", s.handleKeyDelete)
	router.POST("/key", s.handleKeyPost)

	return endless.ListenAndServe(s.addr, router)
}

func (s *Service) handlePing(c *gin.Context) {
	c.String(http.StatusOK, "pong")
}

func (s *Service) handleStats(c *gin.Context) {
	c.JSON(http.StatusOK, s.store.Stats())
}

func (s *Service) handleStore(c *gin.Context) {
	c.JSON(http.StatusOK, s.store.Data())
}

func (s *Service) handleKeyGet(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	level := c.Query("level")

	lvl, err := resolveLevel(level)
	if err != nil {
		c.String(http.StatusBadRequest, "bad request")
	}

	v, err := s.store.Get(key, lvl)
	if err != nil {
		if err == store.ErrNotLeader {
			leader := s.store.LeaderAPIAddr()
			if leader == "" {
				c.String(http.StatusServiceUnavailable, err.Error())
				return
			}

			redirect := s.FormRedirect(c, leader)
			c.Redirect(http.StatusTemporaryRedirect, redirect)
			return
		}

		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{key: v})
}

func (s *Service) handleKeyDelete(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	if err := s.store.Delete(key); err != nil {
		if err == store.ErrNotLeader {
			leader := s.store.LeaderAPIAddr()
			if leader == "" {
				c.String(http.StatusServiceUnavailable, err.Error())
				return
			}

			redirect := s.FormRedirect(c, leader)
			c.Redirect(http.StatusTemporaryRedirect, redirect)
			return
		}

		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.String(http.StatusOK, "Key: %s is deleted ", key)
}

func (s *Service) handleKeyPost(c *gin.Context) {

	m := map[string]string{}
	if err := json.NewDecoder(c.Request.Body).Decode(&m); err != nil {
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	for k, v := range m {

		if err := s.store.Set(k, v); err != nil {
			if err == store.ErrNotLeader {
				leader := s.store.LeaderAPIAddr()
				if leader == "" {
					c.String(http.StatusServiceUnavailable, err.Error())
					return
				}

				redirect := s.FormRedirect(c, leader)
				c.Redirect(http.StatusTemporaryRedirect, redirect)
				return
			}

			c.String(http.StatusInternalServerError, err.Error())
			return
		}

	}

	c.String(http.StatusOK, "Set data success")
}

func (s *Service) handleJoin(c *gin.Context) {

	m := map[string]string{}
	if err := json.NewDecoder(c.Request.Body).Decode(&m); err != nil {
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	if len(m) != 3 {
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	httpAddr, ok := m["httpAddr"]
	if !ok {
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	raftAddr, ok := m["raftAddr"]
	if !ok {
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	nodeID, ok := m["id"]
	if !ok {
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	if err := s.store.Join(nodeID, httpAddr, raftAddr); err != nil {
		if err == store.ErrNotLeader {
			leader := s.store.LeaderAPIAddr()
			if leader == "" {
				c.String(http.StatusServiceUnavailable, err.Error())
				return
			}

			redirect := s.FormRedirect(c, leader)
			c.Redirect(http.StatusTemporaryRedirect, redirect)
			return
		}

		c.String(http.StatusInternalServerError, "system error")
		return
	}

}

// FormRedirect returns the value for the "Location" header for a 301 response.
func (s *Service) FormRedirect(c *gin.Context, host string) string {
	protocol := "http"

	rq := c.Request.URL.RawQuery
	if rq != "" {
		rq = fmt.Sprintf("?%s", rq)
	}
	return fmt.Sprintf("%s://%s%s%s", protocol, host, c.Request.URL.Path, rq)
}

func resolveLevel(lvl string) (store.ConsistencyLevel, error) {
	switch strings.ToLower(lvl) {
	case "default":
		return store.Default, nil
	case "stale":
		return store.Stale, nil
	case "consistent":
		return store.Consistent, nil
	default:
		return store.Default, nil
	}
}
