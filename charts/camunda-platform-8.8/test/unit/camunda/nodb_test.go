// Copyright 2025 Camunda Services GmbH
// Licensed under the Apache License, Version 2.0
// See LICENSE file in the project root for full license information.

package camunda

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestNoDbUnitProperties(t *testing.T) {
	valuesPath := filepath.Join("..", "..", "..", "charts", "camunda-platform-8.8", "test", "integration", "scenarios", "chart-full-setup", "values-integration-test-ingress-nodb.yaml")
	valuesBytes, err := os.ReadFile(valuesPath)
	assert.NoError(t, err)

	var values map[string]interface{}
	assert.NoError(t, yaml.Unmarshal(valuesBytes, &values))

	global := values["global"].(map[string]interface{})
	assert.Equal(t, true, global["noDb"], "global.noDb should be true")

	camunda := values["camunda"].(map[string]interface{})
	db := camunda["database"].(map[string]interface{})
	assert.Equal(t, "none", db["type"], "camunda.database.type should be 'none'")

	connector := camunda["connector"].(map[string]interface{})
	webhook := connector["webhook"].(map[string]interface{})
	assert.Equal(t, false, webhook["enabled"], "camunda.connector.webhook.enabled should be false")
	polling := connector["polling"].(map[string]interface{})
	assert.Equal(t, false, polling["enabled"], "camunda.connector.polling.enabled should be false")
	agenticai := connector["agenticai"].(map[string]interface{})
	assert.Equal(t, false, agenticai["enabled"], "camunda.connector.agenticai.enabled should be false")

	optimize := values["optimize"].(map[string]interface{})
	assert.Equal(t, false, optimize["enabled"], "optimize.enabled should be false")

	webModeler := values["webModeler"].(map[string]interface{})
	play := webModeler["play"].(map[string]interface{})
	assert.Equal(t, false, play["enabled"], "webModeler.play.enabled should be false")
}

