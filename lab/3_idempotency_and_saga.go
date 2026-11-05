package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

var client = &http.Client{Timeout: 10 * time.Second}

func main() {
	_ = godotenv.Load()
	defaultBase := os.Getenv("GATEWAY_URL")
	if defaultBase == "" {
		defaultBase = "http://localhost:8080"
	}
	baseFlag := flag.String("base", defaultBase, "gateway URL")
	flag.Parse()
	base := strings.TrimRight(*baseFlag, "/")
	if _, _, _, err := request(http.MethodGet, base+"/healthz", "", "", nil); err != nil {
		fail("gateway not healthy: %v", err)
	}
	seller := register(base, "seller")
	buyer := register(base, "buyer")
	_, _, body, err := request(http.MethodPost, base+"/listings", seller, "", map[string]any{
		"title": "idempotency-test", "price": 10, "stock": 1,
	})
	if err != nil {
		fail("create listing: %v", err)
	}
	var listing struct {
		ID    string `json:"id"`
		Stock int    `json:"stock"`
	}
	if err := json.Unmarshal(body, &listing); err != nil || listing.ID == "" || listing.Stock != 1 {
		fail("expected listing with stock=1, got: %s", body)
	}
	key := uuid.NewString()
	fmt.Printf("listing=%s initial_stock=1\nIdempotency-Key=%s\n", listing.ID, key)

	type result struct {
		status  int
		replay  bool
		orderID string
		err     error
	}
	results := make(chan result, 50)
	start := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(50)
	for range 50 {
		go func() {
			ready.Done()
			<-start
			status, headers, body, err := request(http.MethodPost, base+"/orders", buyer, key, map[string]any{
				"listing_id": listing.ID, "qty": 1,
			})
			var order struct {
				ID string `json:"id"`
			}
			if err == nil {
				err = json.Unmarshal(body, &order)
				if err == nil && order.ID == "" {
					err = fmt.Errorf("missing order ID: %s", body)
				}
			}
			results <- result{status: status, replay: headers.Get("Idempotent-Replayed") == "true", orderID: order.ID, err: err}
		}()
	}
	ready.Wait()
	started := time.Now()
	close(start)
	statuses := make(map[int]int)
	orderIDs := make(map[string]bool)
	replayed, failures := 0, 0
	for range 50 {
		res := <-results
		statuses[res.status]++
		if res.err != nil {
			failures++
			fmt.Printf("request failed: status=%d error=%v\n", res.status, res.err)
			continue
		}
		orderIDs[res.orderID] = true
		if res.status == http.StatusOK && res.replay {
			replayed++
		}
	}
	fmt.Printf("50 requests in %s\n", time.Since(started).Round(time.Millisecond))
	codes := make([]int, 0, len(statuses))
	for code := range statuses {
		codes = append(codes, code)
	}
	sort.Ints(codes)
	for _, code := range codes {
		fmt.Printf("status %d: %d\n", code, statuses[code])
	}
	fmt.Printf("200 with Idempotent-Replayed: %d\n409: %d\nunique order IDs: %d\n", replayed, statuses[http.StatusConflict], len(orderIDs))
	_, _, body, err = request(http.MethodGet, base+"/listings/"+listing.ID, "", "", nil)
	if err != nil {
		fail("read final stock: %v", err)
	}
	if err := json.Unmarshal(body, &listing); err != nil {
		fail("read final listing: %v", err)
	}
	fmt.Printf("final stock: %d\n", listing.Stock)
	if failures != 0 || statuses[http.StatusCreated] != 1 || replayed != 49 || len(orderIDs) != 1 || listing.Stock != 0 {
		fail("FAILED: expected 1 created, 49 replayed, 1 unique order and stock=0")
	}
	fmt.Println("PASS")
}

func register(base, role string) string {
	_, _, body, err := request(http.MethodPost, base+"/auth/register", "", "", map[string]any{
		"email": "idempotency-" + uuid.NewString() + "@test.local", "password": "hunter22", "role": role,
	})
	if err != nil {
		fail("register %s: %v", role, err)
	}
	var auth struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &auth); err != nil || auth.AccessToken == "" {
		fail("register %s: invalid response: %s", role, body)
	}
	return auth.AccessToken
}

func request(method, url, token, key string, body any) (int, http.Header, []byte, error) {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return 0, nil, nil, err
		}
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err == nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		err = fmt.Errorf("HTTP %d: %s", resp.StatusCode, raw)
	}
	return resp.StatusCode, resp.Header, raw, err
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
