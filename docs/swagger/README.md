# Swagger Documentation for Lunar Rocket Tracking Service

This directory contains auto-generated Swagger/OpenAPI documentation for the Lunar Rocket Tracking Service API.

## Auto-Generated Files

The following files in this directory are automatically generated and **should not be edited manually**:

- `docs.go`: Go code representing the Swagger documentation
- `swagger.json`: JSON representation of the API documentation
- `swagger.yaml`: YAML representation of the API documentation

The `index.html` file is **not** auto-generated and is required for the Swagger UI interface. Do not delete this file.

## Requirements

To regenerate these files, you need:

1 [swag](https://github.com/swaggo/swag): `go install github.com/swaggo/swag/cmd/swag@latest`

## How to Update the Documentation

When you make changes to API comments or structures that are documented with Swagger annotations, follow these steps to update the documentation:

1. Make your code changes, ensuring all API handlers have proper Swagger annotations
2. Run the following command from the project root:

```bash
swag init -g cmd/server/main.go -o docs/swagger
```

This will regenerate all files in this directory. Commit these files along with your code changes.

## Swagger Annotation Examples

For reference, here are examples of proper Swagger annotations:

```go
// HandleMessages processes incoming rocket messages
// @Summary Process rocket messages
// @Description Receives and processes messages from rocket entities
// @Accept json
// @Produce json
// @Param message body models.Envelope true "Rocket message envelope"
// @Success 202 {object} map[string]any "Message accepted"
// @Failure 400 {object} map[string]any "Bad request"
// @Router /messages [post]
func (h *Handler) HandleMessages(w http.ResponseWriter, r *http.Request) {
    // Handler implementation
}
```

## Viewing the Documentation

To view the documentation:

1. Start the Lunar server
2. Navigate to the `/swagger` endpoint in a web browser

For more information about Swagger annotations, visit [Swag Documentation](https://github.com/swaggo/swag)
