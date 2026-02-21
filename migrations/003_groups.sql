-- Запусти эти миграции в своей PostgreSQL базе данных

-- Таблица групповых чатов
CREATE TABLE IF NOT EXISTS group_chats (
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(100) NOT NULL,
    avatar_url TEXT DEFAULT '',
    created_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Участники групп
CREATE TABLE IF NOT EXISTS group_members (
    group_id   INTEGER NOT NULL REFERENCES group_chats(id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       VARCHAR(10) NOT NULL DEFAULT 'member', -- 'admin' или 'member'
    joined_at  TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (group_id, user_id)
);

-- Сообщения в группах
CREATE TABLE IF NOT EXISTS group_messages (
    id         SERIAL PRIMARY KEY,
    group_id   INTEGER NOT NULL REFERENCES group_chats(id) ON DELETE CASCADE,
    sender_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content    TEXT NOT NULL DEFAULT '',
    media_url  TEXT DEFAULT '',
    media_type VARCHAR(20) DEFAULT '',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_group_messages_group_id ON group_messages(group_id);
CREATE INDEX IF NOT EXISTS idx_group_members_user_id ON group_members(user_id);
