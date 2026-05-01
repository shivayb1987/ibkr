package service_test

import (
	"context"
	"github.com/stretchr/testify/assert"
	"ibkr/service"
	"testing"
	"time"
)

func TestNewIBD(t *testing.T) {
	t.Run("checkup", func(t *testing.T) {
		ibdService := service.NewIBD(nil)
		result, err := ibdService.Checkup("AEM")
		assert.NoError(t, err)
		assert.NotEmpty(t, result)
	})
	t.Run("checkQuotes", func(t *testing.T) {
		ibdService := service.NewIBD(nil)
		result, err := ibdService.GetStockQuotes(context.Background(), "SPY", time.Now())
		assert.NoError(t, err)
		assert.NotEmpty(t, result)
	})
}
