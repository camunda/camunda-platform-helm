// Copyright 2022 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
)

const kKeycloakDefaultPort = 18080

func (s *integrationTest) doSessionBasedLogin(loginUrl string, httpClient http.Client) error {
	sessionUrl, err := s.resolveSessionLoginUrl(loginUrl, httpClient)
	if err != nil {
		return err
	}
	s.T().Logf("Send log in request to %s", sessionUrl)

	// log in as demo:demo
	values := url.Values{
		"username": {"demo"},
		"password": {"demo"},
	}
	loginResponse, err := httpClient.PostForm(sessionUrl, values)
	if err != nil {
		return err
	}
	if loginResponse.StatusCode != 200 {
		return errors.New(fmt.Sprintf("On log in expected an 200 status code, but got %d", loginResponse.StatusCode))
	}
	s.T().Logf("Log in at '%s' sucessful!", loginUrl)
	return nil
}

func (s *integrationTest) resolveSessionLoginUrl(loginUrl string, httpClient http.Client) (string, error) {
	// Send request to /auth/login, and follow redirect to keycloak to retrieve the login page.
	// We need to read the returned login page to get the correct URL with session code, only with this session code
	// we are able to log in correctly to keycloak / identity. Additionally, this kind of mimics the user interaction.

	request, err := http.NewRequest("GET", loginUrl, nil)
	if err != nil {
		return "", err
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return "", err
	}

	// The returned login page (from keycloak) is no valid html code, which means we can't use an HTML parser,
	// but we can extract the url via regex.
	//
	// Example form with corresponding URL we are looking for:
	//
	// <form id="kc-form-login" onsubmit="login.disabled = true; return true;"
	//		action="http://localhost:18080/auth/realms/camunda-platform/login-actions/authenticate?session_code=B0BxW2ST2DH0NYE1J-THQncuCVc2yPck5JFmgEnLWbM&amp;execution=be1c2750-2b28-4044-8cf3-22b1331efeae&amp;client_id=camunda-identity&amp;tab_id=tp2zBJnsh6o"
	//		method="post">
	//
	//
	defer response.Body.Close()
	body := response.Body
	b, err := io.ReadAll(body)

	regexCompiled := regexp.MustCompile("(action=\")(.*)(\"[\\s\\w]+=\")")
	match := regexCompiled.FindStringSubmatch(string(b))
	if len(match) < 3 {
		return "", errors.New(fmt.Sprintf("Expected to extract session url from response %s", string(b)))
	}
	sessionUrl := match[2]

	// the url is encoded in the html document, which means we need to replace some characters
	return strings.Replace(sessionUrl, "&amp;", "&", -1), nil
}

func (s *integrationTest) extractJWTTokenFromCookieJar(jar *cookiejar.Jar) (string, error) {
	cookies := jar.Cookies(&url.URL{Scheme: "http", Host: "localhost"})
	identityJWT := "IDENTITY_JWT"
	for _, cookie := range cookies {
		if cookie.Name == identityJWT {
			return cookie.Value, nil
		}
	}
	return "", errors.New("no JWT token found in cookie jar")
}

func (s *integrationTest) doJWTBasedLogin(err error, jar *cookiejar.Jar, identityEndpoint string, httpClient http.Client) error {
	// The previous log in request caused to store a token in our cookie jar.
	// In order to verify whether this token is valid and works with identity we have to extract the token and set
	// the cookie value (JWT token) as authentication header.
	jwtToken, err := s.extractJWTTokenFromCookieJar(jar)
	if err != nil {
		return err
	}
	s.T().Logf("Extracted following JWT token from cookie jar '%s'.", jwtToken)

	verificationUrl := "http://" + identityEndpoint + "/api/clients"
	getRequest, err := http.NewRequest("GET", verificationUrl, nil)
	if err != nil {
		return err
	}
	getRequest.Header.Set("Authentication", "Bearer "+jwtToken)

	// verify the token with the get request
	getResponse, err := httpClient.Do(getRequest)
	if err != nil {
		return err
	}

	if getResponse.StatusCode != 200 {
		return errors.New(fmt.Sprintf("On validating JWT token expected an 200 status code, but got %d", getResponse.StatusCode))
	}
	return nil
}

func (s *integrationTest) doLogin(service string, localPort int, containerPort int) (string, http.Client, func(), error) {
	// In order to login to the service we need to port-forward to Keycloak.
	// The service will redirect (forward) requests to Keycloak to enable the login

	// create keycloak port-forward
	keycloakServiceName := s.resolveKeycloakServiceName()
	_, closeKeycloakPortForward := s.createPortForwardedHttpClientWithPort(keycloakServiceName, kKeycloakDefaultPort)

	// create service port-forward
	serviceName := fmt.Sprintf("%s-%s", s.release, service)
	endpoint, closeFn := s.createPortForwardedHttpClientWithPortAndContainerPort(serviceName, localPort, containerPort)

	coupledCloseFn := func() { closeFn(); closeKeycloakPortForward() }
	httpClient, _, err := s.createHttpClientWithJar()
	if err != nil {
		return "", http.Client{}, coupledCloseFn, err
	}

	err = s.doSessionBasedLogin("http://"+endpoint+"/api/login", httpClient)
	if err != nil {
		return "", http.Client{}, coupledCloseFn, err
	}
	return endpoint, httpClient, coupledCloseFn, nil
}
