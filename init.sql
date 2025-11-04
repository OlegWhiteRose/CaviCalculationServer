\c cavi_db;

CREATE TABLE IF NOT EXISTS auth_user (
    id SERIAL PRIMARY KEY,
    username VARCHAR(150) UNIQUE NOT NULL,
    password VARCHAR(128) NOT NULL,
    is_moderator BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS cavi_groups (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    is_selected BOOLEAN NOT NULL DEFAULT FALSE,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    image_url VARCHAR(500),
    age_group VARCHAR(100) NOT NULL,
    disease_type VARCHAR(100),
    base_price DECIMAL(10,3) NOT NULL DEFAULT 0.000
);

CREATE TABLE IF NOT EXISTS cavi_calculations (
    id SERIAL PRIMARY KEY,
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    creator_login VARCHAR(150) NOT NULL REFERENCES auth_user(username),
    formed_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    moderator_login VARCHAR(150) REFERENCES auth_user(username),
    systolic_pressure INTEGER,
    diastolic_pressure INTEGER,
    pulse_wave_velocity DECIMAL(5,2)
);

CREATE TABLE IF NOT EXISTS cavi_calculation_groups (
    id SERIAL PRIMARY KEY,
    calculation_id INTEGER NOT NULL REFERENCES cavi_calculations(id),
    group_id INTEGER NOT NULL REFERENCES cavi_groups(id),
    cavi_index DECIMAL(10,3) NOT NULL DEFAULT 0.000,
    UNIQUE(calculation_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_cavi_groups_name ON cavi_groups(name);
CREATE INDEX IF NOT EXISTS idx_cavi_groups_age_group ON cavi_groups(age_group);
CREATE INDEX IF NOT EXISTS idx_cavi_groups_disease_type ON cavi_groups(disease_type);
CREATE INDEX IF NOT EXISTS idx_cavi_groups_is_selected ON cavi_groups(is_selected);

CREATE INDEX IF NOT EXISTS idx_cavi_calculations_status ON cavi_calculations(status);
CREATE INDEX IF NOT EXISTS idx_cavi_calculations_creator ON cavi_calculations(creator_login);

CREATE INDEX IF NOT EXISTS idx_cavi_calculation_groups_calc ON cavi_calculation_groups(calculation_id);
CREATE INDEX IF NOT EXISTS idx_cavi_calculation_groups_group ON cavi_calculation_groups(group_id);

INSERT INTO auth_user (username, password, is_moderator) VALUES
('admin', 'pbkdf2_sha256$260000$hash$hash', TRUE),
('moderator', 'pbkdf2_sha256$260000$hash$hash', TRUE),
('user1', 'pbkdf2_sha256$260000$hash$hash', FALSE);

INSERT INTO cavi_groups (name, description, age_group, disease_type, base_price, image_url) VALUES
('Молодые пациенты (до 35 лет)', 'Эластичные сосуды с низким уровнем жесткости', 'young', NULL, 0.000, ''),
('Средний возраст (36–50 лет)', 'Часто появляются первые факторы риска', 'middle', NULL, 0.000, ''),
('Пожилые пациенты (51–70 лет)', 'Повышенная жесткость артерий', 'elderly', NULL, 0.000, ''),
('Молодые пациенты с сахарным диабетом', 'Ранние признаки повреждения сосудов', 'young', 'diabetes', 0.000, ''),
('Средний возраст с сахарным диабетом', 'Ускоренное старение артерий', 'middle', 'diabetes', 0.000, ''),
('Пожилые пациенты с сахарным диабетом', 'Высокий риск сердечно-сосудистых осложнений', 'elderly', 'diabetes', 0.000, ''),
('Молодые пациенты с гипертонией', 'Начальная стадия артериальной жесткости', 'young', 'hypertension', 0.000, ''),
('Средний возраст с гипертонией', 'Факторы риска при гипертонии в среднем возрасте', 'middle', 'hypertension', 0.000, ''),
('Пожилые пациенты с гипертонией', 'Повышенная жесткость при гипертонии в пожилом возрасте', 'elderly', 'hypertension', 0.000, '');
