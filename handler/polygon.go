package handler

import (
	"ibkr/model"
	"net/http"
)

type PolygonService interface {
	Summary(ticker string) (model.PolygonTickerDetails, error)
}

type Polygon struct {
	polygonService PolygonService
}

func NewPolygon(polygonService PolygonService) Polygon {
	return Polygon{
		polygonService: polygonService,
	}
}

func (h Polygon) Summary(w http.ResponseWriter, r *http.Request) {
	ticker := r.URL.Query().Get("ticker")
	data, err := h.polygonService.Summary(ticker)
	if err != nil {
		Errors(w, r, 500, err.Error())
		return
	}

	Success(w, r, data)
}
