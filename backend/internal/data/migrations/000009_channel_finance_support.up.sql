CREATE TABLE pl_channel_service_allowances (
    manufacturer_id BIGINT UNSIGNED NOT NULL,
    reseller_id BIGINT UNSIGNED NOT NULL,
    limit_cents BIGINT UNSIGNED NOT NULL,
    updated_by BIGINT UNSIGNED NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (manufacturer_id, reseller_id),
    FOREIGN KEY (manufacturer_id) REFERENCES pl_organizations(id),
    FOREIGN KEY (reseller_id) REFERENCES pl_organizations(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_channel_finance_requests (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    manufacturer_id BIGINT UNSIGNED NOT NULL,
    reseller_id BIGINT UNSIGNED NOT NULL,
    kind ENUM('topups','invoices','claims') NOT NULL,
    order_id BIGINT UNSIGNED NULL,
    reference VARCHAR(80) COLLATE utf8mb4_bin NULL,
    title VARCHAR(80) NOT NULL,
    amount_cents BIGINT UNSIGNED NOT NULL,
    status ENUM('pending','credited','requested','issued','paid') NOT NULL,
    result_reference VARCHAR(80) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    handled_by BIGINT UNSIGNED NULL,
    handled_at DATETIME(6) NULL,
    UNIQUE (order_id),
    UNIQUE (manufacturer_id, kind, reference),
    INDEX (reseller_id, id),
    INDEX (manufacturer_id, reseller_id, kind),
    FOREIGN KEY (manufacturer_id) REFERENCES pl_organizations(id),
    FOREIGN KEY (reseller_id) REFERENCES pl_organizations(id),
    FOREIGN KEY (order_id) REFERENCES pl_retail_orders(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_channel_service_tickets (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    manufacturer_id BIGINT UNSIGNED NOT NULL,
    reseller_id BIGINT UNSIGNED NOT NULL,
    order_id BIGINT UNSIGNED NULL,
    category VARCHAR(30) NOT NULL,
    subject VARCHAR(80) NOT NULL,
    detail VARCHAR(500) NOT NULL,
    status ENUM('open','resolved','closed') NOT NULL,
    revision INT UNSIGNED NOT NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    request_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    request_hash CHAR(64) CHARACTER SET ascii NOT NULL,
    UNIQUE (created_by, request_id),
    INDEX (reseller_id, id),
    INDEX (manufacturer_id, id),
    FOREIGN KEY (manufacturer_id) REFERENCES pl_organizations(id),
    FOREIGN KEY (reseller_id) REFERENCES pl_organizations(id),
    FOREIGN KEY (order_id) REFERENCES pl_retail_orders(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_channel_ticket_actions (
    ticket_id BIGINT UNSIGNED NOT NULL,
    revision INT UNSIGNED NOT NULL,
    role ENUM('dealer','vendor') NOT NULL,
    action ENUM('resolve','reopen','close') NOT NULL,
    actor_id BIGINT UNSIGNED NOT NULL,
    text VARCHAR(500) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (ticket_id, revision),
    FOREIGN KEY (ticket_id) REFERENCES pl_channel_service_tickets(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
