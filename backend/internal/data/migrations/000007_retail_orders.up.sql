CREATE TABLE pl_retail_offers (
    product_id BIGINT UNSIGNED PRIMARY KEY,
    unit_cents BIGINT UNSIGNED NOT NULL,
    commission_bps INT UNSIGNED NOT NULL,
    active BOOLEAN NOT NULL,
    FOREIGN KEY (product_id) REFERENCES pl_products(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_retail_promotions (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    reseller_id BIGINT UNSIGNED NOT NULL,
    manufacturer_id BIGINT UNSIGNED NOT NULL,
    code CHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    UNIQUE (code),
    UNIQUE (reseller_id, manufacturer_id),
    FOREIGN KEY (reseller_id) REFERENCES pl_organizations(id),
    FOREIGN KEY (manufacturer_id) REFERENCES pl_organizations(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_retail_orders (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    manufacturer_id BIGINT UNSIGNED NOT NULL,
    reseller_id BIGINT UNSIGNED NOT NULL,
    customer_id BIGINT UNSIGNED NOT NULL,
    product_id BIGINT UNSIGNED NOT NULL,
    product_name VARCHAR(150) NOT NULL,
    product_version VARCHAR(100) NOT NULL,
    source ENUM('promotion','assisted') NOT NULL,
    status ENUM('confirmation','payment','settlement','settled','declined') NOT NULL,
    quantity INT UNSIGNED NOT NULL,
    unit_cents BIGINT UNSIGNED NOT NULL,
    commission_bps INT UNSIGNED NOT NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    request_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    request_hash CHAR(64) CHARACTER SET ascii NOT NULL,
    receipt_ref VARCHAR(100) COLLATE utf8mb4_bin NULL,
    payout_ref VARCHAR(100) COLLATE utf8mb4_bin NULL,
    paid_at DATETIME(6) NULL,
    settled_at DATETIME(6) NULL,
    reminded_at DATETIME(6) NULL,
    reminder_count INT UNSIGNED NOT NULL DEFAULT 0,
    created_at DATETIME(6) NOT NULL,
    UNIQUE (created_by, request_id),
    UNIQUE (manufacturer_id, receipt_ref),
    INDEX (manufacturer_id, id),
    INDEX (reseller_id, id),
    INDEX (customer_id, id),
    FOREIGN KEY (manufacturer_id) REFERENCES pl_organizations(id),
    FOREIGN KEY (reseller_id) REFERENCES pl_organizations(id),
    FOREIGN KEY (customer_id) REFERENCES pl_organizations(id),
    FOREIGN KEY (product_id) REFERENCES pl_products(id),
    FOREIGN KEY (created_by) REFERENCES pl_users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
