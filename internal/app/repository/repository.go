package repository

import (
	"fmt"
	"strings"
)

type Repository struct {
}

func NewRepository() (*Repository, error) {
	return &Repository{}, nil
}

type CaviGroup struct {
	ID       int
	Title    string
	Text     string
	ImageURL string
}

type CaviCalculationService struct {
	ID          int
	Title       string
	Subtitle    string
	Intensity   string
	Price       string
	PriceLabel  string
	ImageURL    string
}

type CaviCalculation struct {
	ID       int
	Services []CaviCalculationService
}

func (r *Repository) GetCaviGroups() ([]CaviGroup, error) {
	caviGroups := []CaviGroup{
		{
			ID:       1,
			Title:    "Молодые пациенты (до 35 лет)",
			Text:     "Эластичные сосуды с низким уровнем жесткости",
			ImageURL: "",
		},
		{
			ID:       2,
			Title:    "Средний возраст (36–50 лет)",
			Text:     "Часто появляются первые факторы риска",
			ImageURL: "",
		},
		{
			ID:       3,
			Title:    "Пожилые пациенты (51–70 лет)",
			Text:     "Повышенная жесткость артерий",
			ImageURL: "",
		},
		{
			ID:       4,
			Title:    "Молодые пациенты (до 35 лет) с сахарным диабетом",
			Text:     "Эластичные сосуды с низким уровнем жесткости",
			ImageURL: "",
		},
		{
			ID:       5,
			Title:    "Средний возраст (36–50 лет) с сахарным диабетом",
			Text:     "Эластичные сосуды с низким уровнем жесткости",
			ImageURL: "",
		},
		{
			ID:       6,
			Title:    "Пожилые пациенты (51–70 лет) с сахарным диабетом",
			Text:     "Эластичные сосуды с низким уровнем жесткости",
			ImageURL: "",
		},
		{
			ID:       7,
			Title:    "Молодые пациенты (до 35 лет) с гипертонией",
			Text:     "Эластичные сосуды с низким уровнем жесткости",
			ImageURL: "",
		},
		{
			ID:       8,
			Title:    "Средний возраст (36–50 лет) с гипертонией",
			Text:     "Эластичные сосуды с низким уровнем жесткости",
			ImageURL: "",
		},
		{
			ID:       9,
			Title:    "Пожилые пациенты (51–70 лет) с гипертонией",
			Text:     "Эластичные сосуды с низким уровнем жесткости",
			ImageURL: "",
		},
	}
	if len(caviGroups) == 0 {
		return nil, fmt.Errorf("empty array")
	}

	return caviGroups, nil
}

func (r *Repository) GetCaviGroupsByTitle(title string) ([]CaviGroup, error) {
	caviGroups, err := r.GetCaviGroups()
	if err != nil {
		return []CaviGroup{}, err
	}

	var result []CaviGroup
	for _, caviGroup := range caviGroups {
		if strings.Contains(strings.ToLower(caviGroup.Title), strings.ToLower(title)) {
			result = append(result, caviGroup)
		}
	}

	return result, nil
}

func (r *Repository) GetCaviGroup(id int) (CaviGroup, error) {
	caviGroups, err := r.GetCaviGroups()
	if err != nil {
		return CaviGroup{}, err
	}

	for _, caviGroup := range caviGroups {
		if caviGroup.ID == id {
			return caviGroup, nil
		}
	}
	return CaviGroup{}, fmt.Errorf("группа CAVI не найдена")
}

func (r *Repository) GetCaviCalculations() ([]CaviCalculation, error) {
	requests := []CaviCalculation{
		{
			ID: 1,
			Services: []CaviCalculationService{
				{
					ID:         1,
					Title:      "Средний возраст (36-50 лет) с гипертонией",
					Subtitle:   "Наименование услуги",
					Intensity:  "Умеренная",
					Price:      "9.541",
					PriceLabel: "Рассчитанный CAVI",
					ImageURL:   "",
				},
				{
					ID:         2,
					Title:      "Молодые пациенты (до 35 лет) с сахарным диабетом",
					Subtitle:   "Наименование услуги",
					Intensity:  "Высокая",
					Price:      "8.891",
					PriceLabel: "Рассчитанный CAVI",
					ImageURL:   "",
				},
				{
					ID:         3,
					Title:      "Пожилые пациенты (51-70 лет) с гипертонией",
					Subtitle:   "Наименование услуги",
					Intensity:  "Низкая",
					Price:      "8.891",
					PriceLabel: "Рассчитанный CAVI",
					ImageURL:   "",
				},
			},
		},
	}

    if len(requests) == 0 {
		return nil, fmt.Errorf("empty array")
	}

    return requests, nil
}

func (r *Repository) GetCaviCalculation(id int) (CaviCalculation, error) {
	requests, err := r.GetCaviCalculations()
	if err != nil {
		return CaviCalculation{}, err
	}

	for _, request := range requests {
		if request.ID == id {
			return request, nil
		}
	}
	return CaviCalculation{}, fmt.Errorf("расчёт CAVI не найден")
}
