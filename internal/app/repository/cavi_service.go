package repository

import (
	"fmt"
	"rip/internal/app/ds"
)

func (r *Repository) GetCaviGroups() ([]ds.CaviGroup, error) {
	var groups []ds.CaviGroup
	err := r.db.Where("is_deleted = false").Find(&groups).Error
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
	err := r.db.Where("id = ? AND is_deleted = false", id).First(&group).Error
	if err != nil {
		return ds.CaviGroup{}, err
	}
	return group, nil
}

func (r *Repository) GetCaviGroupsByTitle(title string) ([]ds.CaviGroup, error) {
	var groups []ds.CaviGroup
	err := r.db.Where("name ILIKE ? AND is_deleted = false", "%"+title+"%").Find(&groups).Error
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
	err := r.db.Where("id = ?", groupID).First(&group).Error
	if err != nil {
		return err
	}

	var maxOrder int
	err = r.db.Model(&ds.CaviCalculationGroup{}).Where("calculation_id = ?", calculationID).Select("COALESCE(MAX(order_position), 0)").Scan(&maxOrder).Error
	if err != nil {
		return err
	}

	calculationGroup := ds.CaviCalculationGroup{
		CalculationID: calculationID,
		GroupID:       groupID,
		OrderPosition: maxOrder + 1,
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
	err := r.db.Preload("Group").Where("calculation_id = ?", calculationID).Order("order_position").Find(&groups).Error
	return groups, err
}

