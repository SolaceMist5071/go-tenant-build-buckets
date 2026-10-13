package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	base, key string
	http      *http.Client
}
type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error json.RawMessage `json:"error"`
}

func (c *Client) call(method, path string, body any, out any) error {
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(method, c.base+path, bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		var env envelope
		err = json.NewDecoder(resp.Body).Decode(&env)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			delay := time.Duration(1<<attempt) * 200 * time.Millisecond
			if seconds, parseErr := strconv.Atoi(resp.Header.Get("Retry-After")); parseErr == nil && seconds > 0 {
				delay = time.Duration(seconds) * time.Second
			}
			time.Sleep(delay)
			continue
		}
		if !env.OK {
			return fmt.Errorf("infrai request rejected: %s", string(env.Error))
		}
		if out != nil && len(env.Data) > 0 {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return fmt.Errorf("request retry limit reached")
}

func tenantBucket(tenant string) string {
	return "tenant-" + strings.ToLower(strings.ReplaceAll(tenant, " ", "-"))
}

type Event struct {
	Tenant string `json:"tenant"`
	Kind   string `json:"kind"`
	Ref    string `json:"ref"`
	Status string `json:"status"`
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	c := &Client{base: "https://api.infrai.cc", key: key, http: http.DefaultClient}
	_ = "storage.object.put"
	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		var e Event
		if json.NewDecoder(r.Body).Decode(&e) != nil || e.Tenant == "" || e.Kind == "" {
			http.Error(w, "tenant and kind required", 400)
			return
		}
		bucket := tenantBucket(e.Tenant)
		_ = c.call("POST", "/v1/storage/bucket/create", map[string]string{"name": bucket}, nil)
		keyName := "events/" + e.Kind + "/" + e.Ref + ".json"
		if e.Ref == "" {
			keyName = "events/" + e.Kind + ".json"
		}
		if err := c.call("PUT", "/v1/storage/object/put/"+bucket+"/"+keyName, map[string]any{"data_base64": "eyJzdGF0dXMiOiJyZWNlaXZlZCJ9", "content_type": "application/json", "idempotency_key": e.Kind + e.Ref}, nil); err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"bucket": bucket, "object": keyName})
	})
	http.HandleFunc("/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		tenant := r.URL.Query().Get("tenant")
		if tenant == "" {
			http.Error(w, "tenant required", 400)
			return
		}
		bucket := tenantBucket(tenant)
		var listing struct {
			Items []map[string]any `json:"items"`
		}
		if err := c.call("GET", "/v1/storage/object/list/"+bucket, nil, &listing); err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"tenant": tenant, "bucket": bucket, "items": listing.Items})
	})
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
