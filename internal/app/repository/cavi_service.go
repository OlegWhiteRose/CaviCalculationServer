package repository

import (
	"fmt"
	"gorm.io/gorm"
	"rip/internal/app/ds"
)

func (r *Repository) GetCaviGroups() ([]ds.CaviGroup, error) {
	var groups []ds.CaviGroup
	err := r.db.Where("is_deleted = ?", false).Find(&groups).Error
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("массив пустой")
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

func (r *Repository) GetDraftCalculationByUserID(userID int) (*ds.CaviCalculation, error) {
	var calculation ds.CaviCalculation
	err := r.db.Where("creator_id = ? AND status = ?", userID, ds.StatusDraft).First(&calculation).Error
	if err != nil {
		return nil, err
	}
	return &calculation, nil
}

func (r *Repository) CreateDraftCalculation(userID int) (*ds.CaviCalculation, error) {
	calculation := ds.CaviCalculation{
		Status:    ds.StatusDraft,
		CreatorID: userID,
	}
	err := r.db.Create(&calculation).Error
	if err != nil {
		return nil, err
	}
	return &calculation, nil
}

func (r *Repository) GetCalculationByID(id int) (*ds.CaviCalculation, error) {
	var calculation ds.CaviCalculation
	err := r.db.Where("id = ? AND status != ?", id, ds.StatusDeleted).First(&calculation).Error
	if err != nil {
		return nil, err
	}
	return &calculation, nil
}

func (r *Repository) SoftDeleteCalculation(id int) error {
	err := r.db.Model(&ds.CaviCalculation{}).Where("id = ?", id).Update("status", ds.StatusDeleted).Error
	return err
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
		GroupPrice:    group.BasePrice,
	}

	err = r.db.Create(&calculationGroup).Error
	return err
}

func (r *Repository) RemoveGroupFromCalculation(calculationID, groupID int) error {
	err := r.db.Where("calculation_id = ? AND group_id = ?", calculationID, groupID).Delete(&ds.CaviCalculationGroup{}).Error
	return err
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

type GroupFilters struct {
	Title      string
	AgeGroup   string
	Disease    string
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
	Status    string
	DateFrom  *string
	DateTo    *string
}

func (r *Repository) ListCalculationsFiltered(f CalculationFilters) ([]ds.CaviCalculation, error) {
	q := r.db.Model(&ds.CaviCalculation{}).
		Where("status != ? AND status != ?", ds.StatusDeleted, ds.StatusDraft).
		Preload("Creator").Preload("Moderator")
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.DateFrom != nil && *f.DateFrom != "" {
		q = q.Where("formed_at >= ?", *f.DateFrom)
	}
	if f.DateTo != nil && *f.DateTo != "" {
		q = q.Where("formed_at <= ?", *f.DateTo)
	}
	var items []ds.CaviCalculation
	if err := q.Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) GetCalculationDetailed(id int) (*ds.CaviCalculation, error) {
	var calc ds.CaviCalculation
	if err := r.db.Preload("Creator").Preload("Moderator").First(&calc, "id = ? AND status != ?", id, ds.StatusDeleted).Error; err != nil {
		return nil, err
	}
	return &calc, nil
}

func (r *Repository) UpdateCalculationAllowed(id int, updates map[string]any) error {
	return r.db.Model(&ds.CaviCalculation{}).Where("id = ? AND status != ?", id, ds.StatusDeleted).Updates(updates).Error
}

func (r *Repository) FormCalculation(id int, creatorID int) error {
	return r.db.Model(&ds.CaviCalculation{}).
		Where("id = ? AND creator_id = ? AND status = ?", id, creatorID, ds.StatusDraft).
		Updates(map[string]any{"status": ds.StatusFormed, "formed_at": gorm.Expr("NOW()")}).Error
}

func (r *Repository) CompleteCalculation(id int, moderatorID int) error {
	return r.db.Model(&ds.CaviCalculation{}).
		Where("id = ? AND status = ?", id, ds.StatusFormed).
		Updates(map[string]any{"status": ds.StatusCompleted, "completed_at": gorm.Expr("NOW()"), "moderator_id": moderatorID}).Error
}

func (r *Repository) RejectCalculation(id int, moderatorID int) error {
	return r.db.Model(&ds.CaviCalculation{}).
		Where("id = ? AND status = ?", id, ds.StatusFormed).
		Updates(map[string]any{"status": ds.StatusRejected, "completed_at": gorm.Expr("NOW()"), "moderator_id": moderatorID}).Error
}

func (r *Repository) AddGroupToDraftByUser(userID, groupID int) (*ds.CaviCalculation, error) {
	calc, err := r.GetDraftCalculationByUserID(userID)
	if err != nil {
		calc, err = r.CreateDraftCalculation(userID)
		if err != nil {
			return nil, err
		}
	}
	if err := r.AddGroupToCalculation(calc.ID, groupID); err != nil {
		return nil, err
	}
	return calc, nil
}

func (r *Repository) RemoveGroupFromDraftByUser(userID, groupID int) (*ds.CaviCalculation, error) {
	calc, err := r.GetDraftCalculationByUserID(userID)
	if err != nil {
		return nil, err
	}
	if err := r.RemoveGroupFromCalculation(calc.ID, groupID); err != nil {
		return nil, err
	}
	return calc, nil
}

func (r *Repository) UpdateMMGroupPrice(calculationID, groupID int, price float64) error {
	return r.db.Model(&ds.CaviCalculationGroup{}).
		Where("calculation_id = ? AND group_id = ?", calculationID, groupID).
		Update("group_price", price).Error
}
