-- Drop all memory-related tables in reverse dependency order
DROP TABLE IF EXISTS memory_job;
DROP TABLE IF EXISTS memory_audit_log;
DROP TABLE IF EXISTS user_interaction_event;
DROP TABLE IF EXISTS user_learning_progress;
DROP TABLE IF EXISTS user_memory_item;
DROP TABLE IF EXISTS conversation_state;
DROP TABLE IF EXISTS chat_message;
DROP TABLE IF EXISTS chat_session;
