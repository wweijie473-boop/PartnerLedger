CREATE TABLE pl_channel_partners (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    name VARCHAR(80) NOT NULL,
    owner VARCHAR(60) NOT NULL,
    category VARCHAR(20) NOT NULL,
    source VARCHAR(12) NOT NULL,
    target VARCHAR(12) NOT NULL,
    evidence VARCHAR(500) NOT NULL,
    PRIMARY KEY (id),
    INDEX (organization_id, id),
    FOREIGN KEY (organization_id) REFERENCES pl_organizations(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_channel_opportunities (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    project_id BIGINT UNSIGNED NOT NULL,
    note TEXT NOT NULL,
    note_hash CHAR(64) CHARACTER SET ascii NOT NULL,
    category VARCHAR(20) NOT NULL,
    source VARCHAR(12) NOT NULL,
    target VARCHAR(12) NOT NULL,
    revision INT UNSIGNED NOT NULL,
    stage VARCHAR(16) NOT NULL,
    partner_id BIGINT UNSIGNED NULL,
    follow_up VARCHAR(10) NOT NULL,
    follow_reason VARCHAR(12) NOT NULL,
    fee_amount DECIMAL(10,2) NULL,
    fee_terms VARCHAR(300) NOT NULL,
    outcome VARCHAR(500) NOT NULL,
    contact_reviews JSON NOT NULL,
    rejections JSON NOT NULL,
    PRIMARY KEY (id),
    INDEX (organization_id, id),
    UNIQUE KEY pl_opportunity_project_note (project_id, note_hash),
    FOREIGN KEY (organization_id) REFERENCES pl_organizations(id),
    FOREIGN KEY (project_id) REFERENCES pl_channel_projects(id),
    FOREIGN KEY (partner_id) REFERENCES pl_channel_partners(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_channel_opportunity_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    opportunity_id BIGINT UNSIGNED NOT NULL,
    actor_user_id BIGINT UNSIGNED NOT NULL,
    at DATETIME(6) NOT NULL,
    action VARCHAR(24) NOT NULL,
    label VARCHAR(180) NOT NULL,
    detail JSON NOT NULL,
    PRIMARY KEY (id),
    INDEX (opportunity_id, id),
    FOREIGN KEY (opportunity_id) REFERENCES pl_channel_opportunities(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_channel_opportunity_reads (
    opportunity_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    partner_ids JSON NOT NULL,
    PRIMARY KEY (opportunity_id, user_id),
    FOREIGN KEY (opportunity_id) REFERENCES pl_channel_opportunities(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_channel_partner_cases (
    opportunity_id BIGINT UNSIGNED NOT NULL,
    partner_id BIGINT UNSIGNED NOT NULL,
    category VARCHAR(20) NOT NULL,
    PRIMARY KEY (opportunity_id),
    INDEX (partner_id),
    FOREIGN KEY (opportunity_id) REFERENCES pl_channel_opportunities(id),
    FOREIGN KEY (partner_id) REFERENCES pl_channel_partners(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_channel_visits (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    owner_id BIGINT UNSIGNED NOT NULL,
    project_id BIGINT UNSIGNED NULL,
    opportunity_id BIGINT UNSIGNED NULL,
    text TEXT NOT NULL,
    main TEXT NOT NULL,
    additional TEXT NOT NULL,
    reviewed BOOLEAN NOT NULL,
    confirmed BOOLEAN NOT NULL,
    revision INT UNSIGNED NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    removed_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    INDEX (organization_id, owner_id, id),
    FOREIGN KEY (organization_id) REFERENCES pl_organizations(id),
    FOREIGN KEY (project_id) REFERENCES pl_channel_projects(id),
    FOREIGN KEY (opportunity_id) REFERENCES pl_channel_opportunities(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_channel_visit_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    visit_id BIGINT UNSIGNED NOT NULL,
    actor_user_id BIGINT UNSIGNED NOT NULL,
    at DATETIME(6) NOT NULL,
    action VARCHAR(24) NOT NULL,
    revision INT UNSIGNED NOT NULL,
    PRIMARY KEY (id),
    INDEX (visit_id, id),
    FOREIGN KEY (visit_id) REFERENCES pl_channel_visits(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
