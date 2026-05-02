package scraper

import (
	"strings"

	"github.com/ghlps/poc-go-scraper/internal/models"

	"github.com/PuerkitoBio/goquery"
)

func isMealType(s string) bool {
	return strings.Contains(s, "CAFÉ DA MANHÃ") ||
		strings.Contains(s, "ALMOÇO") ||
		strings.Contains(s, "JANTAR")
}

func parseMeal(part string) (models.Meal, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(part))
	if err != nil {
		return models.Meal{}, err
	}

	name := strings.Join(strings.Fields(doc.Text()), " ")

	icons := []string{}
	doc.Find("img").Each(func(_ int, img *goquery.Selection) {
		if title, exists := img.Attr("title"); exists && title != "" {
			icons = append(icons, title)
		}
	})

	return models.Meal{Name: name, Icons: icons}, nil
}

func parseMealType(htmlContent string) string {
	if strings.Contains(htmlContent, "CAFÉ DA MANHÃ") {
		return "breakfast"
	} else if strings.Contains(htmlContent, "ALMOÇO") {
		return "lunch"
	} else if strings.Contains(htmlContent, "JANTAR") {
		return "dinner"
	}
	return ""
}
