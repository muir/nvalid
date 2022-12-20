package nvalid_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"
	"github.com/muir/nape"
	"github.com/muir/nvalid"
	"github.com/muir/nvelope"
)

const swagger = `
swagger: "2.0"
info:
  version: 1.0.0
  title: testing
schemes:
- "http"

paths:
  /foo/{bar}:
    post:
      summary: Example 
      produces:
        - application/json
      parameters:
        - name: bar
          in: path
          type: number
          required: true
          description: example path parameter
        - name: baz
          in: query
          type: string
          format: email
          required: true
          description: example query parameter
        - name: body
          in: body
          schema:
            type: object
            required: 
              - john
            properties:
              john:
                type: boolean
              betty:
                type: string
          required: true
          description: example body parameter
      responses:
        200:
          description: response
          schema:
            type: object
            required:
              - status
            additionalProperties: false
            properties:
              status:
                type: integer
              weight:
                type: number
        400:
          description: error
          produces:
            - text/plain`

type PostBodyModel struct {
	John  bool    `json:"john"`
	Betty *string `json:"betty"`
}

type ExampleRequestBundle struct {
	Request     PostBodyModel `nvelope:"model"`
	Bar         float64       `nvelope:"path,name=bar"`
	Baz         string        `nvelope:"query,name=baz"`
	ContentType string        `nvelope:"header,name=Content-Type"`
}

type ExampleResponse struct {
	Status interface{} `json:"status,omitempty"`
}

func HandleExampleEndpoint(req ExampleRequestBundle) (nvelope.Response, error) {
	resp := ExampleResponse{
		Status: 100,
	}
	if req.Request.John {
		resp.Status = "string"
	}
	return resp, nil
}

func Service(router *mux.Router) {
	var v2Doc openapi2.T
	err := yaml.Unmarshal([]byte(swagger), &v2Doc)
	if err != nil {
		panic(fmt.Sprint("yaml", err))
	}
	v3Doc, err := openapi2conv.ToV3(&v2Doc)
	if err != nil {
		panic("v3 convert")
	}
	err = v3Doc.Validate(context.Background())
	if err != nil {
		panic("v3 validate")
	}

	requestValidator, responseValidator, err :=
		nvalid.OpenAPI3ValidatorFromParsed(v3Doc, "inline", false)
	if err != nil {
		panic("make validators")
	}
	encodeJSON := nvelope.MakeResponseEncoder("JSON",
		nvelope.WithEncoder("application/json", json.Marshal,
			nvelope.WithAPIEnforcer(responseValidator)))
	service := nape.RegisterServiceWithMux("example", router)
	service.RegisterEndpoint("/foo/{bar}",
		// order matters and this is a correct order
		nvelope.NoLogger,
		nvelope.InjectWriter,
		encodeJSON,
		nvelope.CatchPanic,
		nvelope.Nil204,
		nvelope.ReadBody,
		requestValidator,
		nape.DecodeJSON,
		HandleExampleEndpoint,
	).Methods("POST")
}

// Example shows an injection chain handling a single endpoint using nject,
// nape, and nvelope.
func Example() {
	r := mux.NewRouter()
	Service(r)
	ts := httptest.NewServer(r)
	client := ts.Client()
	doPost := func(url string, body string) {
		// nolint:noctx
		res, err := client.Post(ts.URL+url, "application/json",
			strings.NewReader(body))
		if err != nil {
			fmt.Println("response error:", err)
			return
		}
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			fmt.Println("read error:", err)
			return
		}
		res.Body.Close()
		fmt.Println(res.StatusCode, "->"+string(b))
	}
	fmt.Println("expect valid:")
	doPost("/foo/100?baz=j@example.com", `{"john":false,"betty":"Flinstone"}`)

	fmt.Println("expect request error:")
	doPost("/foo/100", `{"john":false,"betty":"Flinstone"}`) // invalid request

	fmt.Println("expect response error:")
	doPost("/foo/100?baz=j@example.com", `{"john":true,"betty":"Flinstone"}`)

	// Output: expect valid:
	// 200 ->{"status":100}
	// expect request error:
	// 400 ->parameter "baz" in query has an error: value is required but missing
	// expect response error:
	// 500 ->response body doesn't match schema: Error at "/status": field must be set to integer or not be present
	// Schema:
	//   {
	//     "type": "integer"
	//   }
	//
	// Value:
	//   "string"
	//
	//
}
