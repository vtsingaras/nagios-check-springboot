package main

import flag "github.com/spf13/pflag"
import (
	"github.com/Jeffail/gabs"
	"github.com/olorin/nagiosplugin"
	"github.com/parnurzeal/gorequest"
	"crypto/tls"
	"strings"
)

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}


func main() {
	var baseurl string
	var insecure bool
	var username string
	var password string
	var service string
	var ignoreservices []string

	check := nagiosplugin.NewCheck()
	defer check.Finish()

	check.AddResult(nagiosplugin.OK, "All (or some) lights are green :)")

	flag.StringVar(&baseurl, "baseurl", "http://localhost/manage", "Springboot actuator baseurl.")
	flag.BoolVar(&insecure, "insecure", false, "Ignore TLS errors.")
	flag.StringVar(&username, "username", "", "Basic-Auth username.")
	flag.StringVar(&password, "password", "", "Basic-Auth password.")
	flag.StringVar(&service, "service", "", "Check only specified service.")
	flag.StringSliceVar(&ignoreservices, "ignore-services", []string{}, "Ignore these services.")

	flag.Parse()

	if ( username == "" ) != ( password == "" ) {
		check.AddResult(nagiosplugin.UNKNOWN, "<username> and <password> are required together.")
		return
	}

	request := gorequest.New()

	if insecure {
		request.TLSClientConfig(&tls.Config{
			InsecureSkipVerify: true,
		})
	}

	if username != "" {
		request.SetBasicAuth(username, password)
	}


	_, body, errs := request.Get(baseurl + "/health").End()

	if len(errs) > 0 {
		check.AddResult(nagiosplugin.UNKNOWN, "Couldn't fetch from actuator.")
		return
	}


	spring_health, err := gabs.ParseJSON([]byte(body))
	if err != nil {
		check.AddResult(nagiosplugin.UNKNOWN, "Couldn't decode actuator response.")
		return
	}
	children, err := spring_health.ChildrenMap()
	if err != nil {
		check.AddResult(nagiosplugin.UNKNOWN, "Couldn't decode actuator response.")
		return
	}
	var services_down []string
	var services_all []string
	for service_name, service_status := range children {
		switch v := service_status.Data().(type) {
		default:
			continue
		case string:
			continue
		case map[string]interface{}:
			if v["status"].(string) != "UP" {
				services_down = append(services_down, service_name)
			}
			services_all = append(services_all, service_name)
		}
	}


	if service == "" {
		status, ok := spring_health.Path("status").Data().(string)
		if ok == false {
			check.AddResult(nagiosplugin.UNKNOWN, "Couldn't find status in actuator response.")
			return
		}
		if status != "UP" {
			var should_ignore bool = true
			for _, service_down := range services_down {
				if !stringInSlice(service_down, ignoreservices) {
					should_ignore = false
				}
			}
			nagios_out := "Services reported DOWN: " + strings.Join(services_down, ",")
			if should_ignore {
				check.AddResult(nagiosplugin.OK, nagios_out)
			} else {
				check.AddResult(nagiosplugin.CRITICAL, nagios_out)
			}
			return
		}
		check.AddResult(nagiosplugin.OK, "Springboot actuator status is UP.")
		return
	} else {
		if !stringInSlice(service, services_all) {
			nagios_out := "Springboot service " + service + " was not found."
			check.AddResult(nagiosplugin.UNKNOWN, nagios_out)
		}
		if stringInSlice(service, services_down) {
			nagios_out := "Springboot service " + service + " status is DOWN."
			check.AddResult(nagiosplugin.CRITICAL, nagios_out)
		}
		nagios_out := "Springboot service " + service + " status is UP."
		check.AddResult(nagiosplugin.OK, nagios_out)
		return
	}
}
