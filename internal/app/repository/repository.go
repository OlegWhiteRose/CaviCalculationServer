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

type Order struct {
	ID       int
	Title    string
	Text     string
	ImageURL string
}

type CalculationService struct {
	ID          int
	Title       string
	Subtitle    string
	Intensity   string
	Price       string
	PriceLabel  string
	ImageURL    string
}

type Calculation struct {
	ID       int
    Services []CalculationService
}

func (r *Repository) GetOrders() ([]Order, error) {
	orders := []Order{
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
	if len(orders) == 0 {
		return nil, fmt.Errorf("empty array")
	}

	return orders, nil
}

func (r *Repository) GetOrdersByTitle(title string) ([]Order, error) {
	orders, err := r.GetOrders()
	if err != nil {
		return []Order{}, err
	}

	var result []Order
	for _, order := range orders {
		if strings.Contains(strings.ToLower(order.Title), strings.ToLower(title)) {
			result = append(result, order)
		}
	}

	return result, nil
}

func (r *Repository) GetOrder(id int) (Order, error) {
	orders, err := r.GetOrders()
	if err != nil {
		return Order{}, err
	}

	for _, order := range orders {
		if order.ID == id {
			return order, nil
		}
	}
	return Order{}, fmt.Errorf("order not found")
}

func (r *Repository) GetCalculations() ([]Calculation, error) {
    requests := []Calculation{
		{
			ID: 1,
            Services: []CalculationService{
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

func (r *Repository) GetCalculation(id int) (Calculation, error) {
    requests, err := r.GetCalculations()
	if err != nil {
        return Calculation{}, err
	}

	for _, request := range requests {
		if request.ID == id {
			return request, nil
		}
	}
    return Calculation{}, fmt.Errorf("расчёт не найден")
}
