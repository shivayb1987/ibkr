package handler_test

import (
	"github.com/stretchr/testify/assert"
	"ibkr/handler"
	"testing"
)

func TestExtractNumberInt(t *testing.T) {
	val, err := handler.ExtractNumber("1,348.17")
	assert.NoError(t, err)
	assert.Equal(t, 33.5, val)
}

func TestSheetsHandler_AnalyseEntry(t *testing.T) {
}
