UPDATE vehicles
SET plate_number = CASE plate_number
    WHEN '沪环-001' THEN '沪A00001'
    WHEN '沪环-002' THEN '沪A00002'
END
WHERE plate_number IN ('沪环-001', '沪环-002');

INSERT INTO schema_migrations(version, applied_at)
VALUES (2, CURRENT_TIMESTAMP);
