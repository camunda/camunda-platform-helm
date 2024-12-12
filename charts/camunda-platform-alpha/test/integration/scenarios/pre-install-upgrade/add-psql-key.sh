cat << EOF >> ${TEST_VALUES_BASE_DIR}/common/values-integration-test.yaml
postgresql:
  enabled: true
  auth:
    existingSecret: "integration-test-credentials"
    secretKeys:
      adminPasswordKey: "webmodeler-postgresql-admin-password"
      userPasswordKey: "webmodeler-postgresql-user-password"
EOF
