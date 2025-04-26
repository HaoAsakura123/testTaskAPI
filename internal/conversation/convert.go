package conversation

import "fmt"

func ProcessCountryData(countryData interface{}) (string, error) {
    countriesSlice, ok := countryData.([]interface{})
	country := ""
    if !ok {
        return country, fmt.Errorf("expected []interface{}, given %T", countryData)
    }

    converMap := make(map[string]float64)
	var max float64 = 0
    for _, item := range countriesSlice {
        countryMap, ok := item.(map[string]interface{})
        if !ok {
            continue
        }

        countryID, idOk := countryMap["country_id"].(string)
        probability, probOk := countryMap["probability"].(float64)

        if idOk && probOk {
            converMap[countryID] = probability
			if probability > max{
				max = probability
				country = countryID
			}
        }
    }

    if len(converMap) == 0 {
        return country, fmt.Errorf("not found valid country")
    }

    return country, nil
}