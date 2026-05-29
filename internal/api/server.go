package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"nano-indexer/internal/model"

	"github.com/labstack/echo/v4"
)

type transferReader interface {
	Find(ctx context.Context, chainID int64, address string, token string, limit int64) ([]model.TokenTransfer, error)
	AddressSummary(ctx context.Context, chainID int64, address string) (model.AddressSummary, error)
}

func NewServer(transferRepo ...transferReader) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	if len(transferRepo) > 0 && transferRepo[0] != nil {
		repo := transferRepo[0]
		e.GET("/transfers", func(c echo.Context) error {
			chainID, err := strconv.ParseInt(defaultString(c.QueryParam("chain_id"), "1"), 10, 64)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid chain_id"})
			}
			limit, err := strconv.ParseInt(defaultString(c.QueryParam("limit"), "50"), 10, 64)
			if err != nil || limit < 1 || limit > 200 {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "limit must be between 1 and 200"})
			}
			address := strings.ToLower(strings.TrimSpace(c.QueryParam("address")))
			token := strings.ToLower(strings.TrimSpace(c.QueryParam("token")))
			if address == "" && token == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "address or token is required"})
			}
			if address != "" && !isHexAddress(address) {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid address"})
			}
			if token != "" && !isHexAddress(token) {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid token"})
			}
			transfers, err := repo.Find(c.Request().Context(), chainID, address, token, limit)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, transfers)
		})
		e.GET("/addresses/:address/summary", func(c echo.Context) error {
			chainID, err := strconv.ParseInt(defaultString(c.QueryParam("chain_id"), "1"), 10, 64)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid chain_id"})
			}
			address := strings.ToLower(strings.TrimSpace(c.Param("address")))
			if !isHexAddress(address) {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid address"})
			}
			summary, err := repo.AddressSummary(c.Request().Context(), chainID, address)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, summary)
		})
	}

	return e
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func isHexAddress(value string) bool {
	if len(value) != 42 || !strings.HasPrefix(value, "0x") {
		return false
	}
	for _, ch := range value[2:] {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}
