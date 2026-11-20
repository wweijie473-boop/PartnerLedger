CREATE TABLE pl_product_knowledge_profiles (
    product_id BIGINT UNSIGNED NOT NULL,
    category VARCHAR(80) NOT NULL,
    modules JSON NOT NULL,
    deployment_modes JSON NOT NULL,
    license_minimum INT UNSIGNED NULL,
    license_maximum INT UNSIGNED NULL,
    license_note VARCHAR(500) NOT NULL,
    sources JSON NOT NULL,
    updated_by BIGINT UNSIGNED NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (product_id),
    FOREIGN KEY (product_id) REFERENCES pl_products(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_channel_inquiry_drafts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    project_id BIGINT UNSIGNED NOT NULL,
    project_revision INT UNSIGNED NOT NULL,
    product_id BIGINT UNSIGNED NOT NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    requirement JSON NOT NULL,
    summary TEXT NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    INDEX pl_inquiry_draft_actor (project_id, created_by, id),
    FOREIGN KEY (project_id) REFERENCES pl_channel_projects(id) ON DELETE CASCADE,
    FOREIGN KEY (product_id) REFERENCES pl_products(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
