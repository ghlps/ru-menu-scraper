package models

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Restaurant struct {
	Name string         `json:"name" dynamodbav:"name"`
	Code RestaurantCode `json:"code" dynamodbav:"code"`
	Url  string         `json:"url" dynamodbav:"url"`
}

type RestaurantCode int

const (
	UnknownRU RestaurantCode = iota
	POL
	BOT
	CEN
	AGR
)

func NewRestaurant(code RestaurantCode) Restaurant {
	return Restaurant{
		Code: code,
		Name: code.FullName(),
		Url:  code.UrlAddress(),
	}
}

func (r RestaurantCode) String() string {
	switch r {
	case POL:
		return "POL"
	case BOT:
		return "BOT"
	case CEN:
		return "CEN"
	case AGR:
		return "AGR"
	default:
		return "UNKNOWN"
	}
}

func (r RestaurantCode) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String())
}

func (r *RestaurantCode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParseRestaurantCode(s)
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}

func (r RestaurantCode) FullName() string {
	switch r {
	case POL:
		return "Centro Politécnico"
	case BOT:
		return "Jardim Botânico"
	case CEN:
		return "Central"
	case AGR:
		return "Agrárias"
	default:
		return "Unidade Desconhecida"
	}
}

func (r RestaurantCode) UrlAddress() string {
	const base = "https://p4e.ufpr.br/ru	"
	switch r {
	case POL:
		return base + "/cardapio-ru-centro-politecnico/"
	case BOT:
		return base + "/cardapio-ru-jardim-botanico/"
	case CEN:
		return base + "/cardapio-ru-central/"
	case AGR:
		return base + "/cardapio-ru-agrarias/"
	default:
		return base
	}
}

func ParseRestaurantCode(s string) (RestaurantCode, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "POL":
		return POL, nil
	case "BOT":
		return BOT, nil
	case "CEN":
		return CEN, nil
	case "AGR":
		return AGR, nil
	default:
		return UnknownRU, fmt.Errorf("invalid code: %s", s)
	}
}

func (r RestaurantCode) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	return &types.AttributeValueMemberS{Value: r.String()}, nil
}

func (r *RestaurantCode) UnmarshalDynamoDBAttributeValue(av types.AttributeValue) error {
	sv, ok := av.(*types.AttributeValueMemberS)
	if !ok {
		return fmt.Errorf("RestaurantCode must be a string")
	}
	parsed, err := ParseRestaurantCode(sv.Value)
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}
