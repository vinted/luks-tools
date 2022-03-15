package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/thanos-io/thanos/pkg/testutil"
)

type loginApprole struct {
	SecretID string `json:"secret_id"`
	RoleID   string `json:"role_id"`
}

func TestGetKey(t *testing.T) {
	var (
		secretID  string = "secretIDexample"
		roleID    string = "roleIDexample"
		prodToken string = "secretProdTokensss"
		devToken  string = "secretDevToken"
	)

	srv := &http.Server{}
	t.Cleanup(func() {
		_ = srv.Shutdown(context.TODO())
	})
	smux := mux.NewRouter()
	smux.HandleFunc("/v1/auth/approle/login", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(fmt.Sprintf("error occurred: %s", err.Error())))
			return
		}

		var credentials loginApprole
		err = json.Unmarshal(body, &credentials)
		if err != nil {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(fmt.Sprintf("error occurred: %s", err.Error())))
			return
		}

		content, err := json.Marshal(map[string]interface{}{"auth": map[string]string{"client_token": prodToken}})
		if err != nil {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(fmt.Sprintf("error occurred: %s", err.Error())))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if credentials.RoleID == roleID && credentials.SecretID == secretID {
			_, _ = w.Write([]byte(content))
		} else {
			w.WriteHeader(403)
			_, _ = w.Write([]byte("access denied"))
		}
	})

	smux.HandleFunc("/v1/kv/data/luks/testnode", func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(map[string](map[string](map[string]string)){"data": {"data": {"key": "test"}}})
		if err != nil {
			_, _ = w.Write([]byte(fmt.Sprintf("error occurred: %s", err.Error())))
			return
		}
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	testutil.Ok(t, err)

	srv.Handler = smux

	srv.Addr = ":0"
	go func() { _ = srv.Serve(listener) }()

	os.Setenv("VAULT_ADDR", "http://"+listener.Addr().String())
	os.Setenv("VAULT_DEV_ROOT_TOKEN_ID", devToken)
	os.Setenv("LUKS_TOOLS_CFG_PATH", "../../config_sample.yml")

	key, err := GetKey()
	testutil.Ok(t, err)
	if key != "test" {
		t.Fatal("Failed to get key")
	}
}
