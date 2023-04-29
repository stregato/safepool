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
    node VARCHAR(128) NOT NULL, 
    k VARCHAR(64) NOT NULL, 
    s VARCHAR(64) NOT NULL,
    i INTEGER NOT NULL,
    b TEXT,
    CONSTRAINT pk_safe_key PRIMARY KEY(node,k)
);

-- GET_CONFIG
SELECT s, i, b FROM configs WHERE node=:node AND k=:key

-- SET_CONFIG
INSERT INTO configs(node,k,s,i,b) VALUES(:node,:key,:s,:i,:b)
	ON CONFLICT(node,k) DO UPDATE SET s=:s,i=:i,b=:b
	WHERE node=:node AND k=:key

-- DEL_CONFIG
DELETE FROM configs WHERE node=:node

-- INIT
CREATE TABLE IF NOT EXISTS feeds (
    pool VARCHAR(256) NOT NULL, 
    id INTEGER NOT NULL,
    name VARCHAR(8192) NOT NULL, 
    modTime INTEGER NOT NULL,
    size INTEGER NOT NULL,
    authorId VARCHAR(80) NOT NULL,
    hash VARCHAR(128) NOT NULL, 
    meta VARCHAR(4096) NOT NULL,
    slot VARCHAR(16) NOT NULL,
    ctime INTEGER NOT NULL,
    PRIMARY KEY(id)
)

-- INIT
CREATE INDEX IF NOT EXISTS idx_feeds_id ON feeds(id);

-- INIT
CREATE INDEX IF NOT EXISTS idx_feeds_pool ON feeds(pool);

-- INIT
CREATE INDEX IF NOT EXISTS idx_feeds_name ON feeds(name);

-- GET_FEEDS
SELECT id, name, modTime, size, authorId, hash, meta, slot, ctime FROM feeds WHERE pool=:pool AND ctime > :ctime ORDER BY ctime

-- GET_FEED
SELECT id, name, modTime, size, authorId, hash, meta, slot, ctime FROM feeds WHERE pool=:pool AND id=:id

-- SET_FEED
INSERT INTO feeds(pool,id,name,modTime,size,authorId,hash,meta,slot,ctime) VALUES(:pool,:id,:name,:modTime,:size,:authorId,:hash,:meta,:slot,:ctime)

-- DEL_FEED_BEFORE
DELETE FROM feeds WHERE pool=:pool AND id <:beforeId

-- DELETE_FEEDS
DELETE FROM feeds WHERE pool=:pool

-- INIT
CREATE TABLE IF NOT EXISTS keys (
    pool VARCHAR(256) NOT NULL, 
    keyId INTEGER, 
    keyValue VARCHAR(128),
    master INTEGER,
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

-- DELETE_KEYS_SMALLER
DELETE FROM keys WHERE pool=:pool AND keyId < :smallerThan AND NOT master

-- DELETE_KEYS
DELETE FROM keys WHERE pool=:pool

-- SET_MASTER_KEY
UPDATE keys SET master = CASE WHEN keyId=:keyId THEN 1 ELSE 0 END WHERE pool=:pool

-- GET_MASTER_KEY
SELECT keyId, keyValue FROM keys WHERE pool=:pool AND master

-- INIT
CREATE INDEX IF NOT EXISTS idx_keys_master ON keys(master);

-- INIT
CREATE TABLE IF NOT EXISTS pools (
    name VARCHAR(256),
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

-- DELETE_POOL
DELETE FROM pools WHERE name=:name

-- INIT
CREATE TABLE IF NOT EXISTS accesses (
    pool VARCHAR(256),
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

-- DELETE_ACCESSES
DELETE FROM accesses WHERE pool=:pool

-- INIT
CREATE TABLE IF NOT EXISTS chats (
    pool VARCHAR(256),
    chat VARCHAR(128),
    id INTEGER,
    author VARCHAR(128),
    privateId VARCHAR(1024),
    time INTEGER,
    message BLOB,
    CONSTRAINT pk_pool_id_author PRIMARY KEY(pool,chat,id,author)
);

-- INIT
CREATE INDEX IF NOT EXISTS idx_chats_private ON chats(privateId);

-- SET_CHAT_MESSAGE
INSERT INTO chats(pool,chat,id,author,privateId,time,message) VALUES(:pool,:chat,:id,:author,:privateId,:time,:message)
    ON CONFLICT(pool,chat,id,author) DO UPDATE SET message=:message,time=:time,privateId=:privateId
	    WHERE pool=:pool AND chat=:chat AND id=:id AND author=:author

-- GET_CHAT_MESSAGES
SELECT message FROM chats WHERE pool=:pool AND chat=:chat AND time > :after AND time < :before AND privateId=:privateId ORDER BY time DESC LIMIT :limit

-- DELETE_CHAT
DELETE FROM chats WHERE pool=:pool AND chat=:chat

-- GET_CHAT_PRIVATES
SELECT DISTINCT privateId FROM chats WHERE pool=:pool AND chat=:chat

-- INIT
CREATE TABLE IF NOT EXISTS library_files (
    pool VARCHAR(256) NOT NULL,
    base VARCHAR(128) NOT NULL,
    id INTEGER NOT NULL,
    name VARCHAR(4096) NOT NULL,
    authorId VARCHAR(128) NOT NULL,
    modTime INTEGER,
    size INTEGER NOT NULL,
    contentType VARCHAR(128) NOT NULL,
    hash VARCHAR(128) NOT NULL,
    hashChain BLOB,
    ctime INTEGER NOT NULL,
    folder VARCHAR(4096) NOT NULL,
    level INTEGER NOT NULL,
    CONSTRAINT pk_pool_base_id PRIMARY KEY(pool,base,name,authorId)
);

-- SET_LIBRARY_FILE
INSERT INTO library_files(pool,base,id,name,authorId,modTime,size,contentType,hash,hashChain,ctime,folder,level) 
    VALUES(:pool,:base,:id,:name,:authorId,:modTime,:size,:contentType,:hash,:hashChain,:ctime,:folder,:level)
    ON CONFLICT(pool,base,name,authorId) DO UPDATE SET id=:id,modTime=:modTime,size=:size,
    contentType=:contentType,hash=:hash, hashChain=:hashChain,ctime=:ctime
	    WHERE pool=:pool AND base=:base AND name=:name AND authorId=:authorId

-- GET_LIBRARY_FILES_IN_FOLDER
SELECT name,authorId,modTime,id,size,contentType,hash,hashChain,ctime FROM library_files 
    WHERE pool=:pool AND base=:base AND folder=:folder ORDER BY name

-- GET_LIBRARY_FILE_BY_ID
SELECT name,authorId,modTime,id,size,contentType,hash,hashChain,ctime FROM library_files 
    WHERE pool=:pool AND base=:base AND id=:id

-- GET_LIBRARY_FILE_BY_NAME
SELECT name,authorId,modTime,id,size,contentType,hash,hashChain,ctime FROM library_files 
    WHERE pool=:pool AND base=:base AND name=:name AND authorId=:authorId

-- GET_LIBRARY_FILES_SUBFOLDERS
SELECT DISTINCT folder FROM library_files WHERE pool=:pool AND base=:base AND folder LIKE :folder AND level=:level ORDER BY folder

-- GET_LIBRARY_FILES_HASHES
SELECT hash FROM library_files WHERE pool=:pool AND base=:base AND name=:name ORDER BY modTime DESC LIMIT :limit

-- INIT
CREATE TABLE IF NOT EXISTS library_locals (
    pool VARCHAR(256) NOT NULL,
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

-- DELETE_LIBRARY_LOCALS
DELETE FROM library_locals WHERE pool=:pool AND base=:base

-- DELETE_LIBRARY_FILES
DELETE FROM library_files WHERE pool=:pool AND base=:base

-- INIT
CREATE TABLE IF NOT EXISTS invites (
    pool VARCHAR(256) NOT NULL,
    ctime INTEGER NOT NULL,
    valid INTEGER NOT NULL,
    content BLOB NOT NULL
);

-- INIT
CREATE INDEX IF NOT EXISTS idx_invites ON invites(ctime);

-- SET_INVITE
INSERT INTO invites(pool,ctime,valid,content)
    VALUES(:pool,:ctime,:valid,:content)

-- GET_INVITES
SELECT content FROM invites WHERE pool=:pool AND ctime>:ctime

-- GET_INVITES_VALID
SELECT content FROM invites WHERE pool=:pool AND ctime>:ctime AND valid 

-- INIT
CREATE TABLE IF NOT EXISTS breakpoints (
    pool VARCHAR(256) NOT NULL,
    app VARCHAR(128) NOT NULL,
    ctime INTEGER NOT NULL,
    CONSTRAINT pk_breakpoints PRIMARY KEY(pool,app)
);

-- SET_BREAKPOINT
INSERT INTO breakpoints(pool,app,ctime) VALUES(:pool,:app,:ctime)
    ON CONFLICT(pool,app) DO UPDATE SET ctime=:ctime
	    WHERE pool=:pool AND app=:app

-- GET_BREAKPOINT
SELECT ctime FROM breakpoints WHERE pool=:pool AND app=:app

-- INIT
CREATE TABLE IF NOT EXISTS reels (
    pool VARCHAR(256) NOT NULL,
    reel VARCHAR(256) NOT NULL,
    thread VARCHAR(256) NOT NULL,
    id INTEGER NOT NULL,
    name VARCHAR(256) NOT NULL,
    author VARCHAR(128),
    contentType VARCHAR(64),
    ctime INTEGER NOT NULL,
    thumbnail BLOB NOT NULL,
    CONSTRAINT pk_reels PRIMARY KEY(pool,reel,thread,id)
);

-- INIT
CREATE INDEX IF NOT EXISTS idx_reals_ctime ON reels(ctime);

-- SET_REEL
INSERT INTO reels(pool,reel,thread,id,name,author,contentType,ctime,thumbnail)
    VALUES(:pool,:reel,:thread,:id,:name,:author,:contentType,:ctime,:thumbnail)

-- GET_REEL_THREADS
SELECT DISTINCT thread FROM reels WHERE pool=:pool AND reel=:reel

-- GET_REEL
SELECT id,name,contentType,ctime,thumbnail FROM reels WHERE pool=:pool AND reel=:reel 
    AND thread=:thread AND ctime>:from AND ctime<:to
