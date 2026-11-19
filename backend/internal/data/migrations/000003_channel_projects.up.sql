ALTER TABLE pl_organizations MODIFY kind ENUM('MANUFACTURER', 'RESELLER', 'CUSTOMER') NOT NULL;

CREATE TABLE pl_channel_links (
    customer_id BIGINT UNSIGNED NOT NULL,
    reseller_id BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (customer_id, reseller_id),
    INDEX pl_channel_link_reseller (reseller_id, customer_id),
    FOREIGN KEY (customer_id) REFERENCES pl_organizations(id),
    FOREIGN KEY (reseller_id) REFERENCES pl_organizations(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_channel_projects (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    customer_id BIGINT UNSIGNED NOT NULL,
    reseller_id BIGINT UNSIGNED NOT NULL,
    manufacturer_id BIGINT UNSIGNED NULL,
    product_id BIGINT UNSIGNED NULL,
    product_name VARCHAR(150) NOT NULL DEFAULT '',
    title VARCHAR(120) NOT NULL,
    requirement TEXT NOT NULL,
    due_date VARCHAR(10) NOT NULL DEFAULT '',
    stage VARCHAR(32) NOT NULL,
    revision INT UNSIGNED NOT NULL,
    inquiry_id VARCHAR(64) NOT NULL DEFAULT '',
    summary TEXT NOT NULL,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    gap_note TEXT NOT NULL,
    PRIMARY KEY (id),
    INDEX pl_channel_project_customer (customer_id, id),
    INDEX pl_channel_project_reseller (reseller_id, id),
    INDEX pl_channel_project_manufacturer (manufacturer_id, id),
    FOREIGN KEY (customer_id, reseller_id) REFERENCES pl_channel_links(customer_id, reseller_id),
    FOREIGN KEY (manufacturer_id) REFERENCES pl_organizations(id),
    FOREIGN KEY (product_id) REFERENCES pl_products(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_channel_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    project_id BIGINT UNSIGNED NOT NULL,
    actor_user_id BIGINT UNSIGNED NOT NULL,
    actor_org_id BIGINT UNSIGNED NOT NULL,
    at DATETIME(6) NOT NULL,
    scope VARCHAR(16) NOT NULL,
    inquiry_id VARCHAR(64) NOT NULL DEFAULT '',
    label VARCHAR(120) NOT NULL,
    detail TEXT NOT NULL,
    PRIMARY KEY (id),
    INDEX pl_channel_event_project (project_id, id),
    FOREIGN KEY (project_id) REFERENCES pl_channel_projects(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_channel_requests (
    organization_id BIGINT UNSIGNED NOT NULL,
    operation VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    request_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    request_hash CHAR(64) CHARACTER SET ascii NOT NULL,
    project_id BIGINT UNSIGNED NULL,
    result JSON NULL,
    PRIMARY KEY (organization_id, operation, request_id),
    FOREIGN KEY (project_id) REFERENCES pl_channel_projects(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
