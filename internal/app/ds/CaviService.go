package ds

import (
	"math"
)

type CaviGroup struct {
	ID          int     `gorm:"primaryKey"`
	Name        string  `gorm:"type:varchar(255);not null"`
	Description string  `gorm:"type:text"`
	IsSelected  bool    `gorm:"type:boolean not null;default:false"`
	IsDeleted   bool    `gorm:"type:boolean not null;default:false"`
	ImageURL    string  `gorm:"type:varchar(500)"`
	AgeGroup    string  `gorm:"type:varchar(100);not null"`
	DiseaseType *string `gorm:"type:varchar(100)"`
}

func (CaviGroup) TableName() string {
	return "cavi_groups"
}

type CaviCalculation struct {
	ID                int     `gorm:"primaryKey"`
	Status            string  `gorm:"type:varchar(50);not null;default:'draft'"`
	CreatedAt         string  `gorm:"type:varchar(50);not null"`
	FormedAt          *string `gorm:"type:varchar(50)"`
	CompletedAt       *string `gorm:"type:varchar(50)"`
	SystolicPressure  *int
	DiastolicPressure *int
	PulseWaveVelocity *float64
	GroupsCount       *int    `gorm:"column:groups_count"`
	CreatorLogin      string  `gorm:"type:varchar(150);not null" json:"Creator"`
	ModeratorLogin    *string `gorm:"type:varchar(150)" json:"Moderator"`

	Creator           *User                  `gorm:"foreignKey:CreatorLogin;references:Username" json:"-"`
	Moderator         *User                  `gorm:"foreignKey:ModeratorLogin;references:Username" json:"-"`
	CalculationGroups []CaviCalculationGroup `gorm:"foreignKey:CalculationID"`
}

func (CaviCalculation) TableName() string {
	return "cavi_calculations"
}

type CaviCalculationGroup struct {
	ID             int      `gorm:"primaryKey"`
	CalculationID  int      `gorm:"not null;uniqueIndex:idx_calculation_group"`
	GroupID        int      `gorm:"not null;uniqueIndex:idx_calculation_group"`
	CAVIIndex      *float64 `gorm:"column:cavi_index;type:decimal(10,3)"`
	CalculatedCAVI float64  `gorm:"-" json:"-"`

	Group *CaviGroup `gorm:"foreignKey:GroupID"`
}

func (CaviCalculationGroup) TableName() string {
	return "cavi_calculation_groups"
}

// User - без ID, только username как primary key
type User struct {
	Username    string `gorm:"primaryKey;type:varchar(150)" json:"username"`
	Password    string `gorm:"type:varchar(128);not null" json:"-"`
	IsModerator bool   `gorm:"type:boolean;not null;default:false" json:"is_moderator"`
}

func (User) TableName() string { return "auth_user" }

const (
	StatusDraft     = "draft"
	StatusDeleted   = "deleted"
	StatusFormed    = "formed"
	StatusCompleted = "completed"
	StatusRejected  = "rejected"
)

const (
	AgeGroupYoung   = "young"
	AgeGroupMiddle  = "middle"
	AgeGroupElderly = "elderly"
)

const (
	DiseaseTypeDiabetes     = "diabetes"
	DiseaseTypeHypertension = "hypertension"
)

func CalculateCAVI(group *CaviGroup, systolic, diastolic int, pwv float64) float64 {
	if systolic == 0 || diastolic == 0 || pwv == 0 {
		return 0.0
	}

	ps := float64(systolic)
	pd := float64(diastolic)

	if ps <= pd {
		return 0.0
	}

	var M float64
	switch group.AgeGroup {
	case AgeGroupYoung:
		M = 0.9
	case AgeGroupMiddle:
		M = 1.0
	case AgeGroupElderly:
		M = 1.1
	default:
		M = 1.0
	}

	var A float64 = 1.0
	if group.DiseaseType != nil {
		switch *group.DiseaseType {
		case DiseaseTypeDiabetes:
			A = 1.2
		case DiseaseTypeHypertension:
			A = 1.0
		}
	}

	dp := ps - pd
	const rho = 1.05
	cavi := M*(2*rho/dp)*math.Log(ps/pd)*pwv*pwv + A

	return cavi
}
