package fflogs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/machinebox/graphql"
	"github.com/stretchr/testify/assert"
)

func TestClient_Do(t *testing.T) {
	type Response struct {
		ID int `json:"id"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		assert.Equal(t, "bar", reqBody.Variables["foo"])

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": {"id": 1}}`))
	}))
	defer server.Close()

	client := &Client{gqlClient: graphql.NewClient(server.URL)}
	var res Response
	err := client.do(context.Background(), "query { id }", map[string]any{"foo": "bar"}, &res)

	assert.NoError(t, err)
	assert.Equal(t, 1, res.ID)
}
