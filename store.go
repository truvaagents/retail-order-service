package main

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/truvaagents/truva-g3/core"
)

var errNotFound = errors.New("record not found")

type orderStore interface {
	Ready(context.Context) error
	CreateOrder(context.Context, Order) (bool, error)
	ReadOrder(context.Context, string) (Order, error)
	SaveReceipt(context.Context, Receipt) error
	ReadReceipt(context.Context, string) (Receipt, error)
	Index(context.Context, Receipt) error
	ReadFeed(context.Context, Window, int) (Feed, error)
}

type redisStore struct {
	client redis.UniversalClient
	keys   core.RedisKeyspace
}

func (s *redisStore) key(kind, id string) string      { return s.keys.Plain("retail-orders", kind, id) }
func (s *redisStore) Ready(ctx context.Context) error { return s.client.Ping(ctx).Err() }

func (s *redisStore) CreateOrder(ctx context.Context, order Order) (bool, error) {
	data, err := json.Marshal(order)
	if err != nil {
		return false, err
	}
	// A single conditional write commits both the order and its idempotency
	// identity. It needs neither a read-then-write lock nor a multi-key script.
	return s.client.SetNX(ctx, s.key("orders", order.Request.OrderID), data, 0).Result()
}

func (s *redisStore) read(ctx context.Context, kind, id string, out any) error {
	data, err := s.client.Get(ctx, s.key(kind, id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	if len(data) > 32<<10 || json.Unmarshal(data, out) != nil {
		return errors.New("invalid stored record")
	}
	return nil
}
func (s *redisStore) ReadOrder(ctx context.Context, id string) (Order, error) {
	var order Order
	err := s.read(ctx, "orders", id, &order)
	return order, err
}
func (s *redisStore) ReadReceipt(ctx context.Context, id string) (Receipt, error) {
	var receipt Receipt
	err := s.read(ctx, "receipts", id, &receipt)
	return receipt, err
}
func (s *redisStore) SaveReceipt(ctx context.Context, receipt Receipt) error {
	data, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.key("receipts", receipt.ObservationID), data, 24*time.Hour).Err()
}
func (s *redisStore) Index(ctx context.Context, receipt Receipt) error {
	data, err := json.Marshal(Candidate{ObservationID: receipt.ObservationID, Operation: receipt.Operation, At: receipt.At, HTTPStatus: receipt.HTTPStatus, ErrorCode: receipt.ErrorCode})
	if err != nil {
		return err
	}
	key := s.key("observations", "recent")
	if err := s.client.ZAdd(ctx, key, redis.Z{Score: float64(receipt.At.UnixMicro()), Member: string(data)}).Err(); err != nil {
		return err
	}
	return s.client.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(time.Now().Add(-24*time.Hour).UnixMicro(), 10)).Err()
}
func (s *redisStore) ReadFeed(ctx context.Context, window Window, limit int) (Feed, error) {
	values, err := s.client.ZRangeByScore(ctx, s.key("observations", "recent"), &redis.ZRangeBy{Min: strconv.FormatInt(window.Start.UnixMicro(), 10), Max: strconv.FormatInt(window.End.UnixMicro(), 10), Count: int64(limit + 1)}).Result()
	feed := Feed{Window: window, Candidates: []Candidate{}, Truncated: len(values) > limit}
	if err != nil {
		return feed, err
	}
	if feed.Truncated {
		values = values[:limit]
	}
	for _, data := range values {
		var candidate Candidate
		if len(data) > 2048 || json.Unmarshal([]byte(data), &candidate) != nil {
			return feed, errors.New("invalid observation index")
		}
		feed.Candidates = append(feed.Candidates, candidate)
	}
	return feed, nil
}
