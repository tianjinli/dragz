CREATE USER tianjinli WITH PASSWORD '1hhoAYjkW5TArKFmkfxf';

CREATE DATABASE dragz OWNER tianjinli;

GRANT ALL PRIVILEGES ON DATABASE dragz TO tianjinli;

-- optional
GRANT ALL ON SCHEMA public TO tianjinli;
