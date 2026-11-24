ALTER TABLE pl_retail_offers ADD COLUMN annual BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE pl_retail_orders
    ADD COLUMN annual BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN renewal_due_on DATE NULL,
    ADD INDEX pl_retail_renewal_due (reseller_id, renewal_due_on, id);

CREATE TABLE pl_dealer_customer_profiles (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    reseller_id BIGINT UNSIGNED NOT NULL,
    customer_id BIGINT UNSIGNED NULL,
    prospect_name VARCHAR(80) NULL,
    contact VARCHAR(40) NOT NULL,
    note VARCHAR(200) NOT NULL,
    UNIQUE (reseller_id, customer_id),
    UNIQUE (reseller_id, prospect_name),
    FOREIGN KEY (reseller_id) REFERENCES pl_organizations(id),
    FOREIGN KEY (customer_id) REFERENCES pl_organizations(id),
    CHECK ((customer_id IS NULL) <> (prospect_name IS NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_dealer_renewal_notes (
    order_id BIGINT UNSIGNED PRIMARY KEY,
    note VARCHAR(300) NOT NULL,
    contacted_at DATETIME(6) NOT NULL,
    FOREIGN KEY (order_id) REFERENCES pl_retail_orders(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
