package webserver

import (
	"os"
	"testing"
)

func TestMain(t *testing.T) {
	os.Args = []string{"cmd", "-args=2h-valid,2h-valid,2h-valid"}
	main()

	// for _, tc := range tt {
	// 	t.Run(tc.name, func(t *testing.T) {
	// 		request := httptest.NewRequest(tc.method, "/health", strings.NewReader(tc.body))
	// 		responseRecorder := httptest.NewRecorder()

	// 		handler := healthHandler{}
	// 		handler.ServeHTTP(responseRecorder, request)

	// 		if responseRecorder.Code != tc.statusCode {
	// 			t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
	// 		}

	// 		if strings.TrimSpace(responseRecorder.Body.String()) != tc.want {
	// 			t.Errorf("Want '%s', got '%s'", tc.want, responseRecorder.Body)
	// 		}
	// 	})
	// }
}
