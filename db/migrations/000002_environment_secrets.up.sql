CREATE TABLE environment_secrets (
  id CHAR(36) NOT NULL,
  system_id CHAR(36) NOT NULL,
  environment_id CHAR(36) NOT NULL,
  variable_key VARCHAR(128) NOT NULL,
  secret_ref VARCHAR(512) NOT NULL,
  created_by CHAR(36) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_environment_secrets_environment_key (system_id, environment_id, variable_key),
  UNIQUE KEY uk_environment_secrets_system_id_id (system_id, id),
  KEY idx_environment_secrets_created_by (created_by),
  CONSTRAINT fk_environment_secrets_environment
    FOREIGN KEY (system_id, environment_id) REFERENCES environments (system_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_environment_secrets_created_by
    FOREIGN KEY (created_by) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
