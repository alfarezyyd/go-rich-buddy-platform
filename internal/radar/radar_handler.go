package radar

import (
	"net/http"
	"strconv"
	"strings"

	"go-rich-buddy-platform/internal/model"
	"go-rich-buddy-platform/pkg/exception"
	"go-rich-buddy-platform/pkg/helper"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	dbConnection *gorm.DB
	radarService Service
}

func NewHandler(dbConnection *gorm.DB, radarService Service) Controller {
	return &Handler{
		dbConnection: dbConnection,
		radarService: radarService,
	}
}

func (radarHandler *Handler) GetSubSectorRadar(ginContext *gin.Context) {
	subSector := ginContext.DefaultQuery("sub_sector", "Banks")
	userIDStr := ginContext.DefaultQuery("user_id", "1")
	userID, _ := strconv.ParseUint(userIDStr, 10, 64)

	var result *model.RadarResult
	err := radarHandler.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = radarHandler.radarService.GetRadarBySubSector(ginContext.Request.Context(), tx, userID, subSector)
		return err
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))

	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("Subsector radar fetched successfully", result))
}

func (radarHandler *Handler) GetWatchlistRadar(ginContext *gin.Context) {
	userIDStr := ginContext.DefaultQuery("user_id", "1")
	userID, _ := strconv.ParseUint(userIDStr, 10, 64)

	var result *model.RadarResult
	err := radarHandler.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = radarHandler.radarService.GetRadarByWatchlist(ginContext.Request.Context(), tx, userID)
		return err
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))

	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("Watchlist radar fetched successfully", result))
}

func (radarHandler *Handler) GetManualTickersRadar(ginContext *gin.Context) {
	userIDStr := ginContext.DefaultQuery("user_id", "1")
	userID, _ := strconv.ParseUint(userIDStr, 10, 64)

	var request model.ManualTickersRequest
	err := ginContext.ShouldBindJSON(&request)
	if err != nil {
		rawSymbols := ginContext.Query("symbols")
		if rawSymbols != "" {
			request.Symbols = strings.Split(rawSymbols, ",")
		} else {
			helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest))
		}
	}

	var result *model.RadarResult
	err = radarHandler.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = radarHandler.radarService.GetRadarByManualTickers(ginContext.Request.Context(), tx, userID, request.Symbols)
		return err
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))

	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("Manual tickers radar fetched successfully", result))
}

func (radarHandler *Handler) GetDrillDown(ginContext *gin.Context) {
	symbol := ginContext.Param("symbol")
	if symbol == "" {
		symbol = ginContext.Query("symbol")
	}

	var result *model.RadarDrillDownResult
	err := radarHandler.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = radarHandler.radarService.GetDrillDownExplanation(ginContext.Request.Context(), tx, symbol)
		return err
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))

	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("Ticker drilldown explanation fetched successfully", result))
}

func (radarHandler *Handler) SetPreference(ginContext *gin.Context) {
	userIDStr := ginContext.DefaultQuery("user_id", "1")
	userID, _ := strconv.ParseUint(userIDStr, 10, 64)

	var request model.SetRadarPreferenceRequest
	err := ginContext.ShouldBindJSON(&request)
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest))

	var response *model.RadarPreferenceResponse
	err = radarHandler.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		response, err = radarHandler.radarService.SetUserPreference(ginContext.Request.Context(), tx, userID, request)
		return err
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))

	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("User preference updated successfully", response))
}

func (radarHandler *Handler) GetPreference(ginContext *gin.Context) {
	userIDStr := ginContext.DefaultQuery("user_id", "1")
	userID, _ := strconv.ParseUint(userIDStr, 10, 64)

	var response *model.RadarPreferenceResponse
	err := radarHandler.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		response, err = radarHandler.radarService.GetUserPreference(ginContext.Request.Context(), tx, userID)
		return err
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))

	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("User preference fetched successfully", response))
}

func (radarHandler *Handler) GetUserWatchlist(ginContext *gin.Context) {
	userIDStr := ginContext.DefaultQuery("user_id", "1")
	userID, _ := strconv.ParseUint(userIDStr, 10, 64)

	var response *model.WatchlistResponse
	err := radarHandler.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		response, err = radarHandler.radarService.GetUserWatchlist(ginContext.Request.Context(), tx, userID)
		return err
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))

	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("Watchlist fetched successfully", response))
}

func (radarHandler *Handler) AddToWatchlist(ginContext *gin.Context) {
	userIDStr := ginContext.DefaultQuery("user_id", "1")
	userID, _ := strconv.ParseUint(userIDStr, 10, 64)

	var request model.AddWatchlistRequest
	err := ginContext.ShouldBindJSON(&request)
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest))

	var response *model.WatchlistResponse
	err = radarHandler.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		response, err = radarHandler.radarService.AddToWatchlist(ginContext.Request.Context(), tx, userID, request.Symbols)
		return err
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))

	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("Symbols added to watchlist successfully", response))
}

func (radarHandler *Handler) RemoveFromWatchlist(ginContext *gin.Context) {
	userIDStr := ginContext.DefaultQuery("user_id", "1")
	userID, _ := strconv.ParseUint(userIDStr, 10, 64)
	symbol := ginContext.Param("symbol")

	var response *model.WatchlistResponse
	err := radarHandler.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		response, err = radarHandler.radarService.RemoveFromWatchlist(ginContext.Request.Context(), tx, userID, symbol)
		return err
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))

	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("Symbol removed from watchlist successfully", response))
}

func (radarHandler *Handler) TriggerTier1(ginContext *gin.Context) {
	var request model.RunPipelineRequest
	_ = ginContext.ShouldBindJSON(&request)

	response, err := radarHandler.radarService.RunTier1(ginContext.Request.Context(), request.Date)
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusInternalServerError, exception.ErrInternalServerError))

	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("Tier 1 executed successfully", response))
}

func (radarHandler *Handler) TriggerTier2(ginContext *gin.Context) {
	var request model.RunPipelineRequest
	_ = ginContext.ShouldBindJSON(&request)

	response, err := radarHandler.radarService.RunTier2(ginContext.Request.Context(), request.Date)
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusInternalServerError, exception.ErrInternalServerError))

	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("Tier 2 executed successfully", response))
}
