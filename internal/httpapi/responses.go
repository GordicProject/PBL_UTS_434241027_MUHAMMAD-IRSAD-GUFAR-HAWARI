package httpapi

import (
	"encoding/json"
	"log"

	"github.com/gofiber/fiber/v2"
)

// Meta is the pagination meta block returned by list endpoints.
type Meta struct {
	CurrentPage int `json:"current_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
	LastPage    int `json:"last_page"`
}

// successBody is the shape of every successful response.
type successBody struct {
	Success  bool            `json:"success"`
	Message  string          `json:"message"`
	Data     interface{}     `json:"data,omitempty"`
	Meta     *Meta           `json:"meta,omitempty"`
}

// errorBody is the shape of every error response.
type errorBody struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Errors  map[string][]string `json:"errors,omitempty"`
}

// Success writes a uniform success JSON response.
func Success(c *fiber.Ctx, status int, message string, data interface{}, meta *Meta) error {
	c.Status(status)
	return c.JSON(successBody{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

// SuccessNoContent writes a 204 response with an empty body.
func SuccessNoContent(c *fiber.Ctx) error {
	c.Status(fiber.StatusNoContent)
	return c.SendString("")
}

// Error writes a uniform error JSON response.
func Error(c *fiber.Ctx, status int, message string) error {
	c.Status(status)
	return c.JSON(errorBody{
		Success: false,
		Message: message,
	})
}

// ValidationError writes a 422 response with field-level error details.
func ValidationError(c *fiber.Ctx, message string, errors map[string][]string) error {
	c.Status(fiber.StatusUnprocessableEntity)
	return c.JSON(errorBody{
		Success: false,
		Message: message,
		Errors:  errors,
	})
}

// handlePanic is a fiber middleware that recovers panics and returns a 500
// without leaking stack traces.
func handlePanic(c *fiber.Ctx, err error) {
	log.Printf("recovered panic: %v", err)
	_ = c.Status(fiber.StatusInternalServerError).
		JSON(errorBody{Success: false, Message: "Internal server error"})
}

// ErrorHandler is a fiber error handler that converts any unhandled error
// into a 500 JSON response without stack traces.
func ErrorHandler(c *fiber.Ctx, err error) error {
	log.Printf("unhandled error: %v", err)
	_ = c.Status(fiber.StatusInternalServerError).
		JSON(errorBody{Success: false, Message: "Internal server error"})
	return nil
}

// --- helper to parse request bodies safely ---

// bindBody attempts to decode the JSON body into dst. Returns the raw body on
// failure so the caller can decide.
func bindBody(c *fiber.Ctx, dst interface{}) (bool, []byte) {
	data := c.Request().Body()
	if len(data) == 0 {
		return false, nil
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return false, data
	}
	return true, nil
}
