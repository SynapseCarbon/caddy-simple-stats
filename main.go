package caddysimplestats

import (
	"context"
	"net/http"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

func init() {
	caddy.RegisterModule(Stats{})
}

type Stats struct {
	RedisAddr string `json:"redis_addr,omitempty"`
	Prefix    string `json:"prefix,omitempty"`

	rdb *redis.Client
}

func (Stats) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.simple_stats",
		New: func() caddy.Module { return new(Stats) },
	}
}

func (s *Stats) Provision(ctx caddy.Context) error {
	if s.RedisAddr == "" {
		s.RedisAddr = "localhost:6379"
	}
	if s.Prefix == "" {
		s.Prefix = "caddy:"
	}

	s.rdb = redis.NewClient(&redis.Options{
		Addr: s.RedisAddr,
	})

	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	return s.rdb.Ping(pingCtx).Err()
}

func (s *Stats) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	// 1. Maintain global counters
	_ = s.rdb.Incr(ctx, s.Prefix+"total_requests")
	_ = s.rdb.Incr(ctx, s.Prefix+"active_requests")

	// 2. Dynamic URL Tracking: Increment this specific URL path inside a Redis Hash
	// If r.URL.Path is "/about", it increments the field "/about" under the key "prefix:urls"
	_ = s.rdb.HIncrBy(ctx, s.Prefix+"urls", r.URL.Path, 1)

	defer func() {
		_ = s.rdb.Decr(ctx, s.Prefix+"active_requests")
	}()

	return next.ServeHTTP(w, r)
}

func (s *Stats) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "redis_addr":
				if !d.NextArg() {
					return d.ArgErr()
				}
				s.RedisAddr = d.Val()
			case "prefix":
				if !d.NextArg() {
					return d.ArgErr()
				}
				s.Prefix = d.Val()
			}
		}
	}
	return nil
}

var (
	_ caddy.Module                = (*Stats)(nil)
	_ caddyhttp.MiddlewareHandler = (*Stats)(nil)
	_ caddyfile.Unmarshaler       = (*Stats)(nil)
	_ caddy.Provisioner           = (*Stats)(nil)
)
