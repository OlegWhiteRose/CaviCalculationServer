package ds

import (
	"math"
	"time"
)

type CaviGroup struct {
	ID          int       `gorm:"primaryKey"`
	Name        string    `gorm:"type:varchar(255);not null"`
	Description string    `gorm:"type:text"`
	IsDeleted   bool      `gorm:"type:boolean not null;default:false"`
	ImageURL    string    `gorm:"type:varchar(500)"`
	AgeGroup    string    `gorm:"type:varchar(100);not null"`
	DiseaseType *string   `gorm:"type:varchar(100)"`
	BasePrice   float64   `gorm:"type:decimal(10,3);not null;default:0.000"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (CaviGroup) TableName() string {
	return "cavi_groups"
}

type CaviCalculation struct {
	ID                  int        `gorm:"primaryKey"`
	Status              string     `gorm:"type:varchar(50);not null;default:'draft'"`
	CreatedAt           time.Time  `gorm:"not null"`
	FormedAt            *time.Time
	CompletedAt         *time.Time
	ModeratorID         *int
	SystolicPressure    *int
	DiastolicPressure   *int
	PulseWaveVelocity   *float64
	CalculatedCAVI      *float64   `gorm:"type:decimal(5,3)"`
	CreatorID           int        `gorm:"not null"`
	
	Creator             *User                    `gorm:"foreignKey:CreatorID"`
	Moderator           *User                    `gorm:"foreignKey:ModeratorID"`
	CalculationGroups []CaviCalculationGroup `gorm:"foreignKey:CalculationID"`
}

func (CaviCalculation) TableName() string {
	return "cavi_calculations"
}

type CaviCalculationGroup struct {
	ID             int       `gorm:"primaryKey"`
	CalculationID  int       `gorm:"not null;uniqueIndex:idx_calculation_group"`
	GroupID        int       `gorm:"not null;uniqueIndex:idx_calculation_group"`
	OrderPosition  int       `gorm:"not null;default:1"`
	IsMainGroup    bool      `gorm:"type:boolean;default:false"`
	GroupPrice     float64   `gorm:"type:decimal(10,3);not null;default:0.000"`
	CalculatedCAVI float64   `gorm:"-"`
	CreatedAt      time.Time `gorm:"not null"`
	
	Group *CaviGroup `gorm:"foreignKey:GroupID"`
}

func (CaviCalculationGroup) TableName() string {
	return "cavi_calculation_groups"
}

type User struct {
	ID          int       `gorm:"primaryKey"`
	Username    string    `gorm:"type:varchar(150);unique;not null"`
	FirstName   string    `gorm:"type:varchar(150);not null;default:''"`
	LastName    string    `gorm:"type:varchar(150);not null;default:''"`
	Email       string    `gorm:"type:varchar(254);not null;default:''"`
	Password    string    `gorm:"type:varchar(128);not null"`
	IsStaff     bool      `gorm:"type:boolean;not null;default:false"`
	IsActive    bool      `gorm:"type:boolean;not null;default:true"`
	IsSuperuser bool      `gorm:"type:boolean;not null;default:false"`
	DateJoined  time.Time `gorm:"not null"`
	LastLogin   *time.Time
}

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

