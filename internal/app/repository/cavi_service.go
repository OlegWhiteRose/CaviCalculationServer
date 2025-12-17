package repository

import (
	"rip/internal/app/ds"
	"strings"
	"time"

	"gorm.io/gorm"
)

func (r *Repository) GetCaviGroups() ([]ds.CaviGroup, error) {
	var groups []ds.CaviGroup
	err := r.db.Where("is_deleted = ?", false).Find(&groups).Error
	if err != nil {
		return nil, err
	}
	return groups, nil
}

func (r *Repository) GetCaviGroup(id int) (ds.CaviGroup, error) {
	group := ds.CaviGroup{}
	err := r.db.Where("id = ? AND is_deleted = ?", id, false).First(&group).Error
	if err != nil {
		return ds.CaviGroup{}, err
	}
	return group, nil
}

func (r *Repository) GetCaviGroupsByTitle(title string) ([]ds.CaviGroup, error) {
	var groups []ds.CaviGroup
	err := r.db.Where("is_deleted = ? AND name ILIKE ?", false, "%"+title+"%").Find(&groups).Error
	if err != nil {
		return nil, err
	}
	return groups, nil
}

func (r *Repository) GetDraftCalculationByUserLogin(userLogin string) (*ds.CaviCalculation, error) {
	var calculation ds.CaviCalculation
	err := r.db.Where("creator_login = ? AND status = ?", userLogin, ds.StatusDraft).First(&calculation).Error
	if err != nil {
		return nil, err
	}
	return &calculation, nil
}

func (r *Repository) CreateDraftCalculation(userLogin string) (*ds.CaviCalculation, error) {
	calculation := ds.CaviCalculation{
		Status:       ds.StatusDraft,
		CreatorLogin: userLogin,
		CreatedAt:    time.Now().Format("02.01.2006 15:04:05"),
	}
	err := r.db.Create(&calculation).Error
	if err != nil {
		return nil, err
	}
	return &calculation, nil
}

func (r *Repository) GetCalculationByID(id int) (*ds.CaviCalculation, error) {
	var calculation ds.CaviCalculation
	err := r.db.Preload("Creator").Preload("Doctor").Where("id = ?", id).First(&calculation).Error
	if err != nil {
		return nil, err
	}
	return &calculation, nil
}

func (r *Repository) SoftDeleteCalculation(id int) error {
	return r.db.Model(&ds.CaviCalculation{}).Where("id = ?", id).Update("status", ds.StatusDeleted).Error
}

func (r *Repository) AddGroupToCalculation(calculationID, groupID int) error {
	var group ds.CaviGroup
	err := r.db.Where("id = ? AND is_deleted = ?", groupID, false).First(&group).Error
	if err != nil {
		return err
	}

	calculationGroup := ds.CaviCalculationGroup{
		CalculationID: calculationID,
		GroupID:       groupID,
	}

	return r.db.Create(&calculationGroup).Error
}

func (r *Repository) RemoveGroupFromCalculation(calculationID, groupID int) error {
	return r.db.Where("calculation_id = ? AND group_id = ?", calculationID, groupID).Delete(&ds.CaviCalculationGroup{}).Error
}

func (r *Repository) GetCalculationGroups(calculationID int) ([]ds.CaviCalculationGroup, error) {
	var groups []ds.CaviCalculationGroup
	err := r.db.Preload("Group").Where("calculation_id = ?", calculationID).Find(&groups).Error
	return groups, err
}

func (r *Repository) SetGroupSelected(groupID int, selected bool) error {
	return r.db.Model(&ds.CaviGroup{}).Where("id = ?", groupID).Update("is_selected", selected).Error
}

func (r *Repository) UnselectAllGroups() error {
	return r.db.Session(&gorm.Session{AllowGlobalUpdate: true}).
		Model(&ds.CaviGroup{}).
		Update("is_selected", false).Error
}

func (r *Repository) UnselectGroupsByCalculation(calculationID int) error {
	return r.db.Exec(
		"UPDATE cavi_groups SET is_selected = FALSE WHERE id IN (SELECT group_id FROM cavi_calculation_groups WHERE calculation_id = ?)",
		calculationID,
	).Error
}

type GroupFilters struct {
	Title    string
	AgeGroup string
	Disease  string
}

func (r *Repository) GetGroupsFiltered(f GroupFilters) ([]ds.CaviGroup, error) {
	q := r.db.Model(&ds.CaviGroup{}).Where("is_deleted = ?", false)
	if f.Title != "" {
		q = q.Where("name ILIKE ?", "%"+f.Title+"%")
	}
	if f.AgeGroup != "" {
		q = q.Where("age_group = ?", f.AgeGroup)
	}
	if f.Disease != "" {
		q = q.Where("disease_type = ?", f.Disease)
	}
	var groups []ds.CaviGroup
	if err := q.Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

func (r *Repository) CreateGroup(g *ds.CaviGroup) error {
	g.ID = 0
	return r.db.Create(g).Error
}

func (r *Repository) UpdateGroup(id int, updates map[string]any) error {
	return r.db.Model(&ds.CaviGroup{}).Where("id = ? AND is_deleted = ?", id, false).Updates(updates).Error
}

func (r *Repository) SoftDeleteGroup(id int) error {
	return r.db.Model(&ds.CaviGroup{}).Where("id = ?", id).Update("is_deleted", true).Error
}

func (r *Repository) CountItemsInDraft(calculationID int) (int64, error) {
	var c int64
	err := r.db.Model(&ds.CaviCalculationGroup{}).Where("calculation_id = ?", calculationID).Count(&c).Error
	return c, err
}

type CalculationFilters struct {
	Status   string
	DateFrom *string
	DateTo   *string
}

func (r *Repository) ListCalculationsFiltered(f CalculationFilters) ([]ds.CaviCalculation, error) {
	q := r.db.Model(&ds.CaviCalculation{}).Preload("Creator").Preload("Doctor")
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.DateFrom != nil && *f.DateFrom != "" {
		dateFrom := normalizeDate(*f.DateFrom)
		q = q.Where("TO_DATE(SUBSTRING(created_at, 1, 10), 'DD.MM.YYYY') >= TO_DATE(?, 'DD.MM.YYYY')", dateFrom)
	}
	if f.DateTo != nil && *f.DateTo != "" {
		dateTo := normalizeDate(*f.DateTo)
		q = q.Where("TO_DATE(SUBSTRING(created_at, 1, 10), 'DD.MM.YYYY') <= TO_DATE(?, 'DD.MM.YYYY')", dateTo)
	}
	var items []ds.CaviCalculation
	if err := q.Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func normalizeDate(date string) string {
	return strings.ReplaceAll(date, "-", ".")
}

func (r *Repository) GetCalculationDetailed(id int) (*ds.CaviCalculation, error) {
	var calc ds.CaviCalculation
	if err := r.db.Preload("Creator").Preload("Doctor").First(&calc, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &calc, nil
}

func (r *Repository) UpdateCalculationAllowed(id int, updates map[string]any) error {
	return r.db.Model(&ds.CaviCalculation{}).Where("id = ? AND status != ?", id, ds.StatusDeleted).Updates(updates).Error
}

func (r *Repository) FormCalculation(id int) error {
	return r.db.Model(&ds.CaviCalculation{}).
		Where("id = ? AND status = ?", id, ds.StatusDraft).
		Updates(map[string]any{"status": ds.StatusFormed, "formed_at": gorm.Expr("TO_CHAR(NOW(), 'DD.MM.YYYY HH24:MI:SS')")}).Error
}

func (r *Repository) CompleteCalculation(id int, doctorLogin string) error {
	return r.db.Model(&ds.CaviCalculation{}).
		Where("id = ? AND status = ?", id, ds.StatusFormed).
		Updates(map[string]any{"status": ds.StatusCompleted, "completed_at": gorm.Expr("TO_CHAR(NOW(), 'DD.MM.YYYY HH24:MI:SS')"), "doctor_login": doctorLogin}).Error
}

func (r *Repository) RejectCalculation(id int, doctorLogin string) error {
	return r.db.Model(&ds.CaviCalculation{}).
		Where("id = ? AND status = ?", id, ds.StatusFormed).
		Updates(map[string]any{"status": ds.StatusRejected, "completed_at": gorm.Expr("TO_CHAR(NOW(), 'DD.MM.YYYY HH24:MI:SS')"), "doctor_login": doctorLogin}).Error
}

func (r *Repository) AddGroupToDraftByUser(userLogin string, groupID int) (*ds.CaviCalculation, error) {
	calc, err := r.GetDraftCalculationByUserLogin(userLogin)
	if err != nil {
		calc, err = r.CreateDraftCalculation(userLogin)
		if err != nil {
			return nil, err
		}
	}
	if err := r.AddGroupToCalculation(calc.ID, groupID); err != nil {
		return nil, err
	}
	return calc, nil
}

func (r *Repository) RemoveGroupFromDraftByUser(userLogin string, groupID int) (*ds.CaviCalculation, error) {
	calc, err := r.GetDraftCalculationByUserLogin(userLogin)
	if err != nil {
		return nil, err
	}
	if err := r.RemoveGroupFromCalculation(calc.ID, groupID); err != nil {
		return nil, err
	}
	return calc, nil
}

func (r *Repository) UpdateCAVIIndex(calculationID, groupID int, caviIndex float64) error {
	result := r.db.Model(&ds.CaviCalculationGroup{}).
		Where("calculation_id = ? AND group_id = ?", calculationID, groupID).
		Update("cavi_index", caviIndex)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) SetGroupsCount(calculationID int, count int) error {
	return r.db.Model(&ds.CaviCalculation{}).
		Where("id = ?", calculationID).
		Update("groups_count", count).Error
}


// UpdateGroupCAVIIndex обновляет CAVI индекс для группы в заявке (алиас для UpdateCAVIIndex)
func (r *Repository) UpdateGroupCAVIIndex(calculationID, groupID int, caviIndex float64) error {
	return r.UpdateCAVIIndex(calculationID, groupID, caviIndex)
}

// UpdateCalculationGroupsCount обновляет количество групп в заявке
func (r *Repository) UpdateCalculationGroupsCount(calculationID, count int) error {
	return r.SetGroupsCount(calculationID, count)
}

// SetCalculationDoctor устанавливает doctor_login для заявки
func (r *Repository) SetCalculationDoctor(calculationID int, doctorLogin string) error {
	return r.db.Model(&ds.CaviCalculation{}).
		Where("id = ?", calculationID).
		Update("doctor_login", doctorLogin).Error
}

// CompleteCalculationAsync завершает заявку после получения результатов от async сервиса
func (r *Repository) CompleteCalculationAsync(calculationID int) error {
	return r.db.Model(&ds.CaviCalculation{}).
		Where("id = ?", calculationID).
		Updates(map[string]any{
			"status":       ds.StatusCompleted,
			"completed_at": gorm.Expr("TO_CHAR(NOW(), 'DD.MM.YYYY HH24:MI:SS')"),
		}).Error
}
