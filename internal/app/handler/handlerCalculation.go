package handler

import (
	"net/http"
	"rip/internal/app/ds"
	"rip/internal/app/repository"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type calcUpdateReq struct {
	SystolicPressure  *int     `json:"systolic_pressure"`
	DiastolicPressure *int     `json:"diastolic_pressure"`
	PulseWaveVelocity *float64 `json:"pulse_wave_velocity"`
}

type moderateReq struct {
	Action string `json:"action"`
}

// GET иконки корзины (без входных параметров, ид заявки вычисляется)
func (h *Handler) GetCartIconAPI(ctx *gin.Context) {
	userLogin := getCreatorLogin()
	calc, err := h.Repository.GetDraftCalculationByUserLogin(userLogin)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"calculation_id": 0, "items": 0})
		return
	}
	count, err := h.Repository.CountItemsInDraft(calc.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"calculation_id": calc.ID, "items": count})
}

// GET список (кроме удаленных и черновика) с фильтрацией по диапазону даты формирования и статусу
func (h *Handler) ListCalculationsAPI(ctx *gin.Context) {
	status := ctx.Query("status")
	df := ctx.Query("date_from")
	dt := ctx.Query("date_to")
	var dfPtr, dtPtr *string
	if df != "" {
		dfPtr = &df
	}
	if dt != "" {
		dtPtr = &dt
	}
	items, err := h.Repository.ListCalculationsFiltered(repository.CalculationFilters{Status: status, DateFrom: dfPtr, DateTo: dtPtr})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}

	for i := range items {
		groups, err := h.Repository.GetCalculationGroups(items[i].ID)
		if err == nil {
			items[i].CalculationGroups = groups
		}
	}
	ctx.JSON(http.StatusOK, items)
}

// GET одна запись (поля заявки + ее услуги)
func (h *Handler) GetCalculationAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	calc, err := h.Repository.GetCalculationDetailed(id)
	if err != nil || calc == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}
	if calc.Status == ds.StatusDeleted {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}

	groups, err := h.Repository.GetCalculationGroups(calc.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Group != nil && groups[j].Group != nil {
			return groups[i].Group.ID < groups[j].Group.ID
		}
		return groups[i].GroupID < groups[j].GroupID
	})

	for i := range groups {
		if groups[i].Group != nil {
			groups[i].Group.ImageURL = h.Storage.GetImageURLByID(groups[i].Group.ID)
		}
	}
	calc.CalculationGroups = groups
	ctx.JSON(http.StatusOK, calc)
}

// PUT изменения полей заявки по теме
func (h *Handler) UpdateCalculationAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	var req calcUpdateReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid json"})
		return
	}
	updates := map[string]any{}
	if req.SystolicPressure != nil {
		updates["systolic_pressure"] = *req.SystolicPressure
	}
	if req.DiastolicPressure != nil {
		updates["diastolic_pressure"] = *req.DiastolicPressure
	}
	if req.PulseWaveVelocity != nil {
		updates["pulse_wave_velocity"] = *req.PulseWaveVelocity
	}
	if len(updates) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "no fields to update"})
		return
	}
	if err := h.Repository.UpdateCalculationAllowed(id, updates); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// PUT сформировать создателем (дата формирования). Происходит проверка на обязательные поля
func (h *Handler) FormCalculationAPI(ctx *gin.Context) {
	userLogin := getCreatorLogin()
	current, err := h.Repository.GetDraftCalculationByUserLogin(userLogin)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "черновик не найден"})
		return
	}

	id := current.ID
	groups, err := h.Repository.GetCalculationGroups(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	if len(groups) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "в заявке нет услуг"})
		return
	}
	if err := h.Repository.FormCalculation(id); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	_ = h.Repository.UnselectGroupsByCalculation(id)

	calc, _ := h.Repository.GetCalculationByID(id)
	groups, _ = h.Repository.GetCalculationGroups(id)
	for i := range groups {
		if groups[i].Group != nil {
			groups[i].Group.ImageURL = h.Storage.GetImageURLByID(groups[i].Group.ID)
		}
	}
	calc.CalculationGroups = groups
	ctx.JSON(http.StatusOK, calc)
}

// PUT завершить/отклонить модератором
func (h *Handler) ModerateCalculationAPI(ctx *gin.Context) {
	if !isModeratorLoggedIn() {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "fail", "message": "требуется роль модератора"})
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	var req moderateReq
	if err := ctx.BindJSON(&req); err != nil || (req.Action != "complete" && req.Action != "reject") {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid action"})
		return
	}
	moderatorLogin := getModeratorLogin()
	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}
	if current.Status != ds.StatusFormed {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "модерация доступна только для сформированной заявки"})
		return
	}
	if req.Action == "complete" {
		groups, err := h.Repository.GetCalculationGroups(id)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
			return
		}
		if err := h.Repository.CompleteCalculation(id, moderatorLogin); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
			return
		}
		if err := h.Repository.SetGroupsCount(id, len(groups)); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
			return
		}
		calc, _ := h.Repository.GetCalculationByID(id)
		groups, _ = h.Repository.GetCalculationGroups(id)
		for i := range groups {
			if groups[i].Group != nil {
				groups[i].Group.ImageURL = h.Storage.GetImageURLByID(groups[i].Group.ID)
			}
		}
		calc.CalculationGroups = groups
		ctx.JSON(http.StatusOK, calc)
		return
	}
	if err := h.Repository.RejectCalculation(id, moderatorLogin); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	calc, _ := h.Repository.GetCalculationByID(id)
	groups, _ := h.Repository.GetCalculationGroups(id)
	for i := range groups {
		if groups[i].Group != nil {
			groups[i].Group.ImageURL = h.Storage.GetImageURLByID(groups[i].Group.ID)
		}
	}
	calc.CalculationGroups = groups
	ctx.JSON(http.StatusOK, calc)
}

// PUT завершить модератором (отдельный метод)
func (h *Handler) CompleteCalculationAPI(ctx *gin.Context) {
	if !isModeratorLoggedIn() {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "fail", "message": "требуется роль модератора"})
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	moderatorLogin := getModeratorLogin()

	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}
	if current.Status != ds.StatusFormed {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "завершение доступно только для сформированной заявки"})
		return
	}

	groups, err := h.Repository.GetCalculationGroups(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	var total float64
	for _, g := range groups {
		if g.CAVIIndex != nil {
			total += *g.CAVIIndex
		}
	}
	if err := h.Repository.CompleteCalculation(id, moderatorLogin); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	if err := h.Repository.SetGroupsCount(id, len(groups)); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"total_cavi_index": total, "groups_count": len(groups)})
}

// PUT отклонить модератором (отдельный метод)
func (h *Handler) RejectCalculationAPI(ctx *gin.Context) {
	if !isModeratorLoggedIn() {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "fail", "message": "требуется роль модератора"})
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	moderatorLogin := getModeratorLogin()

	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}
	if current.Status != ds.StatusFormed {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "отклонение доступно только для сформированной заявки"})
		return
	}
	if err := h.Repository.RejectCalculation(id, moderatorLogin); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DELETE удаление (дата формирования)
func (h *Handler) DeleteCalculationAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	userLogin := getCreatorLogin()
	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}
	if current.Status != ds.StatusDraft || current.CreatorLogin != userLogin {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "удаление доступно только для черновика создателя"})
		return
	}

	_ = h.Repository.UnselectGroupsByCalculation(id)
	if err := h.Repository.SoftDeleteCalculation(id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GET создание заявки (HTML)
func (h *Handler) GetCaviCalculation(ctx *gin.Context) {
	userLogin := "user1"

	calculation, err := h.Repository.CreateDraftCalculation(userLogin)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to create calculation"})
		return
	}

	ctx.Redirect(http.StatusFound, "/calculations/"+strconv.Itoa(calculation.ID))
}

// GET одна запись (HTML)
func (h *Handler) GetCaviCalculationByID(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid calculation ID"})
		return
	}

	calculation, err := h.Repository.GetCalculationByID(id)
	if err != nil || calculation == nil {
		logrus.Error(err)
		ctx.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Calculation not found"})
		return
	}

	if calculation.Status == ds.StatusDeleted {
		ctx.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Calculation not found or deleted"})
		return
	}

	defaultSystolic := 120
	defaultDiastolic := 80
	defaultPWV := 8.5

	calculationGroups, err := h.Repository.GetCalculationGroups(calculation.ID)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to fetch calculation groups"})
		return
	}

	sort.Slice(calculationGroups, func(i, j int) bool {
		if calculationGroups[i].Group != nil && calculationGroups[j].Group != nil {
			return calculationGroups[i].Group.ID < calculationGroups[j].Group.ID
		}
		return calculationGroups[i].GroupID < calculationGroups[j].GroupID
	})

	for i := range calculationGroups {
		if calculationGroups[i].Group != nil {
			calculationGroups[i].Group.ImageURL = h.Storage.GetImageURLByID(calculationGroups[i].Group.ID)

			cavi := ds.CalculateCAVI(
				calculationGroups[i].Group,
				defaultSystolic,
				defaultDiastolic,
				defaultPWV,
			)
			calculationGroups[i].CalculatedCAVI = cavi
		}
	}

	calculation.CalculationGroups = calculationGroups

	ctx.HTML(http.StatusOK, "cavi-calculation.html", gin.H{
		"caviCalculation":  calculation,
		"defaultSystolic":  defaultSystolic,
		"defaultDiastolic": defaultDiastolic,
		"defaultPWV":       defaultPWV,
		"defaultImage":     h.Storage.GetDefaultImageURL(),
	})
}

// POST удаление заявки (HTML form)
func (h *Handler) SoftDeleteCalculationByID(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid calculation ID"})
		return
	}

	if err := h.Repository.UnselectAllGroups(); err != nil {
		logrus.Error(err)
	}

	err = h.Repository.SoftDeleteCalculation(id)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to delete calculation"})
		return
	}

	ctx.Redirect(http.StatusFound, "/")
}
