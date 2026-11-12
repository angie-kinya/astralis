package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// a malformed id must stop at the router as a 404, never reach postgres as a cast error
func TestIDParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := map[string]bool{
		"3f2b8c1e-9a4d-4e2b-8f1a-2c3d4e5f6a7b": true,
		"3F2B8C1E-9A4D-4E2B-8F1A-2C3D4E5F6A7B": true,
		"not-a-uuid":                           false,
		"":                                     false,
		"3f2b8c1e-9a4d-4e2b-8f1a-2c3d4e5f6a7b'; --": false,
	}
	for in, want := range cases {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: in}}
		got, ok := idParam(c, "id")
		require.Equal(t, want, ok, in)
		if want {
			require.Equal(t, in, got)
		} else {
			require.Equal(t, http.StatusNotFound, w.Code, in)
		}
	}
}
