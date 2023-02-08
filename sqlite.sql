-- INIT
CREATE TABLE IF NOT EXISTS identities (
    id VARCHAR(256),
    i64 BLOB,
    trusted INTEGER,
    alias VARCHAR(256),
    PRIMARY KEY(id)
);

-- INIT
CREATE INDEX IF NOT EXISTS idx_identities_trust ON identities(trusted);

-- GET_IDENTITIES
SELECT i64, alias FROM identities

-- GET_IDENTITY
SELECT i64, alias FROM identities WHERE id=:id

-- GET_TRUSTED
SELECT i64, alias FROM identities WHERE trusted

-- SET_TRUSTED
UPDATE identities SET trusted=:trusted WHERE id=:id

-- SET_IDENTITY
INSERT INTO identities(id,i64,alias) VALUES(:id,:i64,'')
    ON CONFLICT(id) DO UPDATE SET i64=:i64
	WHERE id=:id

-- SET_ALIAS
UPDATE identities SET alias=:alias WHERE id=:id

-- INIT
CREATE TABLE IF NOT EXISTS configs (
    pool VARCHAR(128) NOT NULL, 
    k VARCHAR(64) NOT NULL, 
    s VARCHAR(64) NOT NULL,
    i INTEGER NOT NULL,
    b TEXT,
    CONSTRAINT pk_safe_key PRIMARY KEY(pool,k)
);

-- GET_CONFIG
SELECT s, i, b FROM configs WHERE pool=:pool AND k=:key

-- SET_CONFIG
INSERT INTO configs(pool,k,s,i,b) VALUES(:pool,:key,:s,:i,:b)
	ON CONFLICT(pool,k) DO UPDATE SET s=:s,i=:i,b=:b
	WHERE pool=:pool AND k=:key

-- INIT
CREATE TABLE IF NOT EXISTS feeds (
    offset INTEGER PRIMARY KEY AUTOINCREMENT,
    pool VARCHAR(128) NOT NULL, 
    id INTEGER NOT NULL,
    name VARCHAR(8192) NOT NULL, 
    modTime INTEGER NOT NULL,
    size INTEGER NOT NULL,
    authorId VARCHAR(80) NOT NULL,
    hash VARCHAR(128) NOT NULL, 
    meta VARCHAR(4096) NOT NULL,
    slot VARCHAR(16) NOT NULL
)

-- INIT
CREATE INDEX IF NOT EXISTS idx_feeds_id ON feeds(id);

-- INIT
CREATE INDEX IF NOT EXISTS idx_feeds_pool ON feeds(pool);

-- INIT
CREATE INDEX IF NOT EXISTS idx_feeds_name ON feeds(name);

-- GET_FEEDS
SELECT id, name, modTime, size, authorId, hash, offset, meta, slot FROM feeds WHERE pool=:pool AND offset > :offset ORDER BY offset

-- GET_FEED
SELECT id, name, modTime, size, authorId, hash, offset, meta, slot FROM feeds WHERE pool=:pool AND id=:id

-- SET_FEED
INSERT INTO feeds(pool,id,name,modTime,size,authorId,hash,meta,slot) VALUES(:pool,:id,:name,:modTime,:size,:authorId,:hash,:meta,:slot)

-- DEL_FEED_BEFORE
DELETE FROM feeds WHERE pool=:pool AND id <:beforeId

-- INIT
CREATE TABLE IF NOT EXISTS keys (
    pool VARCHAR(128) NOT NULL, 
    keyId INTEGER, 
    keyValue VARCHAR(128),
    CONSTRAINT pk_safe_keyId PRIMARY KEY(pool,keyId)
);

-- GET_KEYS
SELECT keyId, keyValue FROM keys WHERE pool=:pool

-- GET_KEY
SELECT keyValue FROM keys WHERE pool=:pool AND keyId=:keyId

-- SET_KEY
INSERT INTO keys(pool,keyId,keyValue) VALUES(:pool,:keyId,:keyValue)
    ON CONFLICT(pool,keyId) DO UPDATE SET keyValue=:keyValue
	    WHERE pool=:pool AND keyId=:keyId

-- INIT
CREATE TABLE IF NOT EXISTS pools (
    name VARCHAR(512),
    configs BLOB,
    PRIMARY KEY(name)
);

-- GET_POOL
SELECT configs FROM pools WHERE name=:name

-- LIST_POOL
SELECT DISTINCT name FROM pools

-- SET_POOL
INSERT INTO pools(name,configs) VALUES(:name,:configs)
    ON CONFLICT(name) DO UPDATE SET configs=:configs
	    WHERE name=:name

-- INIT
CREATE TABLE IF NOT EXISTS slots (
    pool VARCHAR(512),
    exchange VARCHAR(4096),
    slot VARCHAR(16),
    CONSTRAINT pk_pool_exchange PRIMARY KEY(pool,exchange)
);

-- SET_SLOT
INSERT INTO slots(pool,exchange,slot) VALUES(:pool,:exchange,:slot)
    ON CONFLICT(pool,exchange) DO UPDATE SET slot=:slot
	    WHERE pool=:pool AND exchange=:exchange

-- GET_SLOT
SELECT slot FROM slots WHERE pool=:pool AND exchange=:exchange

-- INIT
CREATE TABLE IF NOT EXISTS accesses (
    pool VARCHAR(128),
    id VARCHAR(256),
    state INTEGER,
    modTime INTEGER,
    ts INTEGER,
    CONSTRAINT pk_safe_sig_enc PRIMARY KEY(pool,id)
);

-- GET_TRUSTED_ACCESSES
SELECT s.id, i.i64, state, modTime, ts FROM identities i INNER JOIN accesses s WHERE s.pool=:pool AND (i.id = s.id OR i.id IS NULL) AND i.trusted

-- GET_ACCESSES
SELECT s.id, i.i64, state, modTime, ts FROM identities i INNER JOIN accesses s WHERE s.pool=:pool AND (i.id = s.id OR i.id IS NULL)

-- GET_ACCESS
SELECT state, modTime, ts FROM accesses s WHERE s.pool=:pool AND id = :id 

-- SET_ACCESS
INSERT INTO accesses(pool,id,state,modTime,ts) VALUES(:pool,:id,:state,:modTime,:ts)
    ON CONFLICT(pool,id) DO UPDATE SET state=:state,modTime=:modTime,ts=:ts WHERE
    pool=:pool AND id=:id

-- DEL_GRANT
DELETE FROM accesses WHERE id=:id AND pool=:pool

-- INIT
CREATE TABLE IF NOT EXISTS chats (
    pool VARCHAR(128),
    id INTEGER,
    author VARCHAR(128),
    message BLOB,
    offset INTEGER,
    CONSTRAINT pk_pool_id_author PRIMARY KEY(pool,id,author)
);

-- SET_CHAT_MESSAGE
INSERT INTO chats(pool,id,author,message,offset) VALUES(:pool,:id,:author,:message, :offset)
    ON CONFLICT(pool,id,author) DO UPDATE SET message=:message
	    WHERE pool=:pool AND id=:id AND author=:author

-- GET_CHAT_MESSAGES
SELECT message FROM chats WHERE pool=:pool AND id > :afterId AND id < :beforeId ORDER BY id DESC LIMIT :limit

-- GET_CHATS_OFFSET
SELECT max(offset) FROM chats WHERE pool=:pool

-- INIT
CREATE TABLE IF NOT EXISTS library_files (
    pool VARCHAR(128) NOT NULL,
    base VARCHAR(128) NOT NULL,
    id INTEGER NOT NULL,
    name VARCHAR(4096) NOT NULL,
    authorId VARCHAR(128) NOT NULL,
    modTime INTEGER,
    size INTEGER NOT NULL,
    contentType VARCHAR(128) NOT NULL,
    hash VARCHAR(128) NOT NULL,
    hashChain BLOB,
    offset INTEGER NOT NULL,
    folder VARCHAR(4096) NOT NULL,
    level INTEGER NOT NULL,
    CONSTRAINT pk_pool_base_id PRIMARY KEY(pool,base,name,authorId)
);

-- SET_LIBRARY_FILE
INSERT INTO library_files(pool,base,id,name,authorId,modTime,size,contentType,hash,hashChain,offset,folder,level) 
    VALUES(:pool,:base,:id,:name,:authorId,:modTime,:size,:contentType,:hash,:hashChain,:offset,:folder,:level)
    ON CONFLICT(pool,base,name,authorId) DO UPDATE SET id=:id,modTime=:modTime,size=:size,
    contentType=:contentType,hash=:hash, hashChain=:hashChain,offset=:offset
	    WHERE pool=:pool AND base=:base AND name=:name AND authorId=:authorId

-- GET_LIBRARY_FILES_IN_FOLDER
SELECT name,authorId,modTime,id,size,contentType,hash,hashChain,offset FROM library_files 
    WHERE pool=:pool AND base=:base AND folder=:folder ORDER BY name

-- GET_LIBRARY_FILE_BY_ID
SELECT name,authorId,modTime,id,size,contentType,hash,hashChain,offset FROM library_files 
    WHERE pool=:pool AND base=:base AND id=:id

-- GET_LIBRARY_FILE_BY_NAME
SELECT name,authorId,modTime,id,size,contentType,hash,hashChain,offset FROM library_files 
    WHERE pool=:pool AND base=:base AND name=:name AND authorId=:authorId

-- GET_LIBRARY_FILES_SUBFOLDERS
SELECT folder FROM library_files WHERE pool=:pool AND base=:base AND folder LIKE :folder AND level=:level ORDER BY folder

-- GET_LIBRARY_FILES_HASHES
SELECT hash FROM library_files WHERE pool=:pool AND base=:base AND name=:name ORDER BY modTime DESC LIMIT :limit

-- GET_LIBRARY_FILES_OFFSET
SELECT max(offset) FROM library_files WHERE pool=:pool AND base=:base 

-- INIT
CREATE TABLE IF NOT EXISTS library_locals (
    pool VARCHAR(128) NOT NULL,
    base VARCHAR(128) NOT NULL,
    folder VARCHAR(4096) NOT NULL,
    name VARCHAR(4096) NOT NULL,
    path VARCHAR(4096) NOT NULL,
    id INTEGER NOT NULL,
    authorId VARCHAR(128) NOT NULL,
    modTime INTEGER,
    size INTEGER NOT NULL,
    hash VARCHAR(128) NOT NULL,
    hashChain BLOB,
    CONSTRAINT pk_pool_base_name PRIMARY KEY(pool,base,name)
);

-- SET_LIBRARY_LOCAL
INSERT INTO library_locals(pool,base,folder,name,path,id,authorId,modTime,size,hash,hashChain)
    VALUES(:pool,:base,:folder,:name,:path,:id,:authorId,:modTime,:size,:hash,:hashChain)
    ON CONFLICT(pool,base,name) DO UPDATE SET id=:id,modTime=:modTime,authorId=:authorId,size=:size,path=:path,
        hash=:hash,hashChain=:hashChain
	    WHERE pool=:pool AND base=:base AND name=:name

-- GET_LIBRARY_LOCALS_IN_FOLDER
SELECT name,path,id,authorId,modTime,size,hash,hashChain FROM library_locals WHERE pool=:pool AND base=:base AND folder=:folder

-- GET_LIBRARY_LOCAL
SELECT name,path,id,authorId,modTime,size,hash,hashChain FROM library_locals WHERE pool=:pool AND base=:base AND name=:name
