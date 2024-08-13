package main

import (
	"os"
	"fmt"
	"bufio"
	"bytes"
	"strings"
	"time"
	"encoding/json"

	"math/rand/v2"
)

type CoreData struct {
	Category string
	Items    []string
}

func parseBuffer(buffer []byte) []CoreData {
	var result []CoreData
	scanner := bufio.NewScanner(bytes.NewReader(buffer))
	var currentCategory CoreData

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if currentCategory.Category != "" {
				result = append(result, currentCategory)
				currentCategory = CoreData{}
			}
		} else if currentCategory.Category == "" {
			currentCategory.Category = line
		} else {
			currentCategory.Items = append(currentCategory.Items, line)
		}
	}

	if currentCategory.Category != "" {
		result = append(result, currentCategory)
	}

	return result
}


func ReadCoreData(filename string) []CoreData {
	content, _ := os.ReadFile(filename)
	return parseBuffer(content)
}


type Brand struct {
	Id int
	Name string
}

type Franchise struct {
	Id int
	Name string
}

type Category struct {
	Id int
	Name string
}

type Country struct {
	Id int
	Name string
}

type City struct {
	Id int
	Name string
	CountryId int
}

type Product struct {
	Id int
	Name string
	CategoryId int
	BrandId int
}

type Store struct {
	Id int
	FranchiseId int
	CityId int
	Name string
}

// AKA StockUnit
type StockUnit struct {
	Id int
	ProductId int
	StoreId int
}

type SimpleDate struct {
	Year int
	Month int
	Day int
}

type StockTransaction struct {
	Id int
	StockUnitId int
	IsSale bool
	Count int
	TotalPrice int // in cents!
	Date SimpleDate
}

type Database struct {
	Brands     []Brand

	Franchises []Franchise

	Categories []Category
	Products   []Product

	Countries  []Country
	Cities     []City

	Stores []Store

	StockUnits []StockUnit

	// FACTS!
	StockTransactions []StockTransaction
}

const MIN_PRICE = 100

func adjustedPrice(r *rand.Rand, price int) int {
	var adjustmentRange = 2
	if price > 20_00 {
		adjustmentRange = int(float64(price) * 0.15)
	}
	var adjustment = r.IntN(adjustmentRange + adjustmentRange) - adjustmentRange
	newPrice := price + adjustment
	if newPrice < MIN_PRICE {
		return MIN_PRICE
	} else {
		return newPrice
	}
}

func main() {
	r := rand.New(rand.NewPCG(10, 12))

	categories := ReadCoreData("categories.txt")
	countries := ReadCoreData("countries.txt")
	companies := ReadCoreData("companies.txt")
	_ = categories
	_ = countries

	var db Database
	for i, n := range companies[0].Items {
		db.Brands = append(db.Brands, Brand{
			Id: i,
			Name: n,
		})
	}
	for i, n := range companies[1].Items {
		db.Franchises = append(db.Franchises, Franchise{
			Id: i,
			Name: n,
		})
	}

	for i, c := range countries {
		db.Countries = append(db.Countries, Country{
			Id: i,
			Name: c.Category,
		})
		for _, city := range c.Items {
			db.Cities = append(db.Cities, City{
				Id: len(db.Cities),
				Name: city,
				CountryId: i,
			})
		}
	}

	for i, c := range categories {
		db.Categories = append(db.Categories, Category{
			Id: i,
			Name: c.Category,
		})
		for _, product := range c.Items {
			for _, brand := range db.Brands {
				if r.Float64() < 0.3 {
					db.Products = append(db.Products, Product {
						Id: len(db.Products),
						Name: fmt.Sprintf("%s | %s", product, brand.Name),
						CategoryId: i,
						BrandId: brand.Id,
					})
				}
			}
		}
	}

	// cross franchise with city to generate stores
	for _, franchise := range db.Franchises {
		for _, city := range db.Cities {
			count := r.IntN(3) // equal chance of: no store, just one store, multiple stores
			if count == 2 { // if multiple, allow up to 4
				count = 2 + r.IntN(3)
			}
			for i := 1; i <= count; i++ {
				suffix := ""
				if count > 1 {
					suffix = fmt.Sprintf(" (#%d)", i)
				}
				name := fmt.Sprintf("%s %s%s", franchise.Name, city.Name, suffix)
				db.Stores = append(db.Stores, Store {
					Id: len(db.Stores),
					Name: name,
					FranchiseId: franchise.Id,
					CityId: city.Id,
				})
			}
		}
	}

	// cross stores with products to make store items!
	for _, store := range db.Stores {
		for _, product := range db.Products {
			if r.Float64() < 0.4 {
				db.StockUnits = append(db.StockUnits, StockUnit{
					Id: len(db.StockUnits),
					ProductId: product.Id,
					StoreId: store.Id,
				})
			}
		}
	}

	// generate the sales facts!! This will blow up the file size!
	// generate a base price per product (randomly)
	basePrice := make(map[int]int)
	for i := range db.Products {
		basePrice[i] = 100 + r.IntN(100_00)
	}

	currentStockPrice := make(map[int]int)

	startDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 0, 10)
	for s := startDate; s.Before(endDate); s = s.AddDate(0, 0, 1) {
		year, month, day := s.Date()
		stockUnitIds := r.Perm(len(db.StockUnits))
		for _, stockUnitId := range stockUnitIds {

			// skip half the items or more
			if r.Float64() < 0.9 {
				continue
			}

			stockUnit := &db.StockUnits[stockUnitId]
			currentPrice, ok := currentStockPrice[stockUnitId]
			if !ok {
				currentPrice = adjustedPrice(r, basePrice[stockUnit.ProductId])
				currentStockPrice[stockUnitId] = currentPrice
			}

			// randomly adjust the price every once in a while
			if r.Float64() < 0.05 {
				currentPrice = adjustedPrice(r, currentStockPrice[stockUnit.ProductId])
				currentStockPrice[stockUnitId] = currentPrice
			}

			count := r.IntN(10)

			db.StockTransactions = append(db.StockTransactions, StockTransaction{
				Id: len(db.StockTransactions),
				StockUnitId: stockUnitId,
				Count: count,
				TotalPrice: count * currentPrice,
				IsSale: true,
				Date: SimpleDate{ year, int(month), day },
			})
		}
	}


	if true {
		out, _ := os.Create("generated.json")
		defer out.Close()
		enc := json.NewEncoder(out)
		enc.SetIndent("", "    ")
		enc.Encode(db)
	}
}