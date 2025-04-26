package getAPI

import (
	"encoding/json"
	"fmt"
	"net/http"
	_ "strconv"
	cnvr "testTaskAPI/internal/conversation"

)

type PersonInfo struct{
	Age int `json:"age"`
	Gender string `json:"gender"`
	Country interface{} `json:"country"`
	Name string `json:"name"`
}

type ResponsePersonInfo struct{
	Age int `json:"age"`
	Gender string `json:"gender"`
	Country string `json:"country"`
	Name string `json:"name"`
}


func GetAPI(name string) (ResponsePersonInfo, error) {
    services := []string{
        "https://api.agify.io/?name=",
        "https://api.genderize.io/?name=",
        "https://api.nationalize.io/?name=",
    }
    
    var information PersonInfo
    information.Name = name
    
    for _, baseURL := range services {
        url := baseURL + name
        resp, err := http.Get(url)
        if err != nil {
            continue
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            continue
        }

        var partialInfo PersonInfo
        if err := json.NewDecoder(resp.Body).Decode(&partialInfo); err != nil {
            continue
        }
        
        information = mergeInformation(information, partialInfo)
    }
	//fmt.Println(information.Country)
    if information.Age == 0 && information.Gender == "" && information.Country == nil {
        return ResponsePersonInfo{}, fmt.Errorf("no data received from APIs")
    }

	contryStr, err := cnvr.ProcessCountryData(information.Country)
	if err != nil{
		return ResponsePersonInfo{}, fmt.Errorf("no data received from APIs country")
	}
	response := &ResponsePersonInfo{Age: information.Age, Gender: information.Gender, Country: contryStr}
	
	// нужно придумать как у интерфейса определить тип и вычленить информацию
    return *response, nil
}

func mergeInformation(old, new PersonInfo) PersonInfo {
	result := old

	if result.Age == 0 && new.Age != 0 {
		result.Age = new.Age
	}

	if result.Gender == "" && new.Gender != "" {
		result.Gender = new.Gender
	}

	if new.Country != nil{
		result.Country = new.Country
	}
	return result
}
