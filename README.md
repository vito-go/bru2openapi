# bru2openapi

**bru2openapi** is a Go command-line tool that converts `.bru` files into OpenAPI 3.0 YAML specifications. It supports environments, authentication, parameters, request bodies, and multiple content types. Designed for developers who want to automatically generate API documentation from `.bru` files.

> This tool works with `.bru` files created by [Bruno](https://github.com/usebruno/bruno), a new and innovative API client, aimed at revolutionizing the status quo represented by Postman and similar tools out there.

---

## Features

- Convert `.bru` files into OpenAPI 3.0 YAML
- Support for:
  - Query and header parameters
  - Request bodies:
    - `application/json`
    - `application/x-www-form-urlencoded`
    - `multipart/form-data`
    - `text/plain` or `application/xml`
  - Authentication:
    - Basic
    - API Key
    - Bearer Token
    - Digest
    - WSSE
- Automatically generate tags based on folder structure
- Environment support for dynamic server URLs
- Exclude specific folders from processing
- CLI with configurable input/output paths

---

## Installation

Install the latest version directly with Go:

```bash
go install github.com/vito-go/bru2openapi@latest
 
# Alternatively, clone the repository and build manually:
 
git clone https://github.com/vito-go/bru2openapi.git
cd bru2openapi
go build -o bru2openapi main.go
```

Make sure your Go bin directory is in your `PATH`:

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

---

## Usage

```bash
bru2openapi -d ./bru_files -o ./openapi.yaml -exclude tmp,examples
```

### CLI Flags

| Flag       | Description                                 | Default          |
| ---------- | ------------------------------------------- | ---------------- |
| `-d`       | Path to root folder containing `.bru` files | `./bru_files`    |
| `-o`       | Output path for generated OpenAPI YAML      | `./openapi.yaml` |
| `-exclude` | Comma-separated list of folders to skip     | (none)           |

---

## Example `.bru` Folder Structure

```
bru_files/
├─ users/
│  ├─ get_user.bru
│  └─ update_user.bru
└─ environments/
   └─ dev.bru
```

* Folder names become tags in OpenAPI (`users/`)
* Environment blocks (`environments/dev.bru`) generate server URLs

---

## Example `.bru` File

```
get {
  url: /users/{id}
}

params:query {
  id: 123
}

body:json {
  "name": "Alice",
  "age": 30
}

auth:apikey {
  key: api_key
  value: token
  placement: header
}
```

### Generated OpenAPI Path

```yaml
paths:
  /users/{id}:
    get:
      summary: get_user
      parameters:
        - name: id
          in: query
          required: true
          example: 123
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                  description: Alice
                age:
                  type: number
                  description: 30
      responses:
        "200":
          description: 'OK'
      security:
        - apiKey: []
```

---

## Viewing the OpenAPI YAML

After generating the YAML:

1. Open [Swagger Editor](https://editor.swagger.io/)
2. Click `File → Import File`
3. Select the generated `openapi.yaml`
4. Explore and interact with your API documentation

You can also use **Redoc**, **Postman**, or other OpenAPI tools to visualize and test endpoints.

---

## Quick Start Workflow

```
.bru files -> bru2openapi CLI -> openapi.yaml -> Swagger / Redoc / Postman / Scalar
```

---

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature-name`)
3. Commit your changes (`git commit -m "Add new feature"`)
4. Push to your branch (`git push origin feature-name`)
5. Open a Pull Request

---

## License

This project is licensed under the **MIT License**.



