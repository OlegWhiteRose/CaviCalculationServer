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

type RequestService struct {
	ID          int
	Title       string
	Subtitle    string
	Intensity   string
	Price       string
	PriceLabel  string
}

type Request struct {
	ID       int
	Services []RequestService
}

func (r *Repository) GetOrders() ([]Order, error) {
	orders := []Order{
		{
			ID:       1,
			Title:    "Молодые пациенты (до 35 лет)",
			Text:     "Эластичные сосуды с низким уровнем жесткости",
			ImageURL: "young-patients.jpg",
		},
		{
			ID:       2,
			Title:    "Средний возраст (36–50 лет)",
			Text:     "Часто появляются первые факторы риска",
			ImageURL: "middle-age.jpg",
		},
		{
			ID:       3,
			Title:    "Пожилые пациенты (51–70 лет)",
			Text:     "Повышенная жесткость артерий",
			ImageURL: "elderly-patients.jpg",
		},
		{
			ID:       4,
			Title:    "Молодые пациенты (до 35 лет) с сахарным диабетом",
			Text:     "Эластичные сосуды с низким уровнем жесткости",
			ImageURL: "young-diabetes.jpg",
		},
		{
			ID:       5,
			Title:    "Средний возраст (36–50 лет) с сахарным диабетом",
			Text:     "Эластичные сосуды с низким уровнем жесткости",
			ImageURL: "middle-diabetes.jpg",
		},
		{
			ID:       6,
			Title:    "Пожилые пациенты (51–70 лет) с сахарным диабетом",
			Text:     "Эластичные сосуды с низким уровнем жесткости",
			ImageURL: "elderly-diabetes.jpg",
		},
		{
			ID:       7,
			Title:    "Молодые пациенты (до 35 лет) с гипертонией",
			Text:     "Эластичные сосуды с низким уровнем жесткости",
			ImageURL: "young-hypertension.jpg",
		},
		{
			ID:       8,
			Title:    "Средний возраст (36–50 лет) с гипертонией",
			Text:     "Эластичные сосуды с низким уровнем жесткости",
			ImageURL: "middle-hypertension.jpg",
		},
		{
			ID:       9,
			Title:    "Пожилые пациенты (51–70 лет) с гипертонией",
			Text:     "Эластичные сосуды с низким уровнем жесткости",
			ImageURL: "elderly-hypertension.jpg",
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

func (r *Repository) GetRequests() ([]Request, error) {
	requests := []Request{
		{
			ID: 1,
			Services: []RequestService{
				{
					ID:         1,
					Title:      "Средний возраст (36-50 лет) с гипертонией",
					Subtitle:   "Наименование услуги",
					Intensity:  "Умеренная",
					Price:      "9.541",
					PriceLabel: "Рассчитанный САИ",
				},
				{
					ID:         2,
					Title:      "Молодые пациенты (до 35 лет) с сахарным диабетом",
					Subtitle:   "Наименование услуги",
					Intensity:  "Высокая",
					Price:      "8.891",
					PriceLabel: "Рассчитанный САИ",
				},
				{
					ID:         3,
					Title:      "Пожилые пациенты (51-70 лет) с гипертонией",
					Subtitle:   "Наименование услуги",
					Intensity:  "Низкая",
					Price:      "8.891",
					PriceLabel: "Рассчитанный САИ",
				},
			},
		},
	}

	if len(requests) == 0 {
		return nil, fmt.Errorf("empty array")
	}

	return requests, nil
}

func (r *Repository) GetRequest(id int) (Request, error) {
	requests, err := r.GetRequests()
	if err != nil {
		return Request{}, err
	}

	for _, request := range requests {
		if request.ID == id {
			return request, nil
		}
	}
	return Request{}, fmt.Errorf("request not found")
}
