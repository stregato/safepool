# Intro
Safepool is a distributed secure add-only content distribution based on passive storage. It comes both as a Go and binary library. 

The pillars of the technology are:
1. Data is stored on storage, which is partitioned by domains. Users are identified by a private/public EC key
2. Data is encrypted on with AES256. Each domain has a different password.
2. The AES password is encrypted with the public key of each user. The encrypted password is kept in the users' file
3. When a new user joins the domain, his identity and related encrypted password is added to the users' file. When a user is removed from a domain, a new password is generated and shared with all users except him
4. Both users' file and changes files are valid when they are signed by a trusted user. 
A change file that is not trusted is ignored, while a users' file that is not trusted prevents futher operations


# Key Concepts

## User
A user is a person that intends to distribute data. A user is identified by a public/private key (ed25519). By extension a user is a software process run (potentially in background) under the identity of a user.

## Local Storage
The local storage is a memory space on a device owned by a user. For performance reasons usually data is kept in a local storage

## Exchange
An exchange is a location where data and changes are stored so to be asynchronously shared across users. 
An exchange is implemented with existing technologies, such as SFTP, S3 and Azure Storage.


## Users
A user is identified by a pair private/public key. A user can be either an admin or a follower.
- admin: he can add/remove users
- follower: he can share and download data but he cannot add/remove users

## Domain
A domain identifies the users who can share specific data. Each user in a domain is identified by its private/public key.
Domains have a hierarchical structure similar to Internet domains (e.g. public.safepool.zone). In the future this hierarchy may be used for shared access


## Lineage
It is the sequence of changes applied to a file since its creation. 

## Beauty Context
If multiple users modify the same file, the lineage has a fork. When a fork is present, a user must choose his favorite version by downloading it. The vote 

## Access
Data located in transport is subject to access control. Since transport cannot offer active control, the access is passive by encryption. 

_While all clients that access a exchanger can see the content, only entitled clients can decrypt specific content_

In fact each file must be encrypted with a simmetric key (_AES256_)

## Synchronization
This is the core operation when a client receives updates from the network and uploads possible changes. It is defined in multiple phases

### 1. Local discovery
Files for each domain are checked against information on the DB. If no information about the file is on the DB, the hash is calculated so to check for rename cases. 
In case of rename, the record is update with the status _UPDATE_.

If the DB already contains information about the file, the modification time and the content are checked looking for changes; in case of changes, the status is set to _UPDATE_.

### 2. Remote discovery
A client connect to the closest exchange (round-robin latency is used) available. Then files are filtered based on the Snowfallid, ignoring all files that are older according to the logs in the DB.

For each change file:
- the client rebuilds the chain of changes 


# API

```
  func Start() 
  func Join(token string) error
  func AddExchange(domain string, exchange json) error
  func GetPublic() string
  func ListDomains() []string
  func State(domain string) []string
  func Watch(func handler(string))
```

# Console Protocol

Safepool 
- Helo: provide server information
- State: list domains and status
- State [domain]: list files in a domain 
- Add [domain/file]: add the file to the stage for the next push
- Mon: monitor updates from all domains
- AddDomain [invite]: add a new domain 
- NewDomain [invite]: create a new domain


## Samples
| Request | Response | 
|------|----|
| HELO | WESHARE 1.0 |
| STATE | public.safepool.zone <br> test.safepool.zone |
| STATE test.safepool.zone | sample.txt C-<br>other.txt U- |
| ADD test.safepool.zone/

# Design
- Layer1: Storage
- Layer2: Access
- Layer3: Feeds


## Local 
Each client keeps some information locally. Most data is stored in a SQLite db.

### TABLE Config
contains configuration parameters both at global and domain level

| Field | Type | Constraints | Description |
|------|----|----|-----------|
| domain | VARCHAR(128) |  | Domain the configuration refers to. When the config is global, the value is NULL |
| key | VARCHAR(64) | NOT NULL | Key of the config |
| value | VARCHAR(64) | NOT NULL | Value of the config |


The following config parameters are supported:
- identity.public: public key of the user
- identity.private: private key of the user
- 

### TABLE Changes
tracks all change coming from the net

| Field | Type | Constraints | Description |
|------|----|----|-----------|
| domain | VARCHAR(128) |  | Domain |
| name | VARCHAR(128) |  | Full path of the file |
| hash | CHAR(64) | NOT NULL | Hash of the file |
| change | VARCHAR(16) | NOT NULL | Change file on the network |


### TABLE Keys
tracks all change coming from the net

| Field | Type | Constraints | Description |
|------|----|----|-----------|
| domain | VARCHAR(128) |  | Domain |
| thread | VARCHAR(128) |  | Full path of the file |
| id | CHAR(64) | NOT NULL | Hash of the file |
| value | VARCHAR(16) | NOT NULL | Change file on the network |

### TABLE Identities
Track known user and the trust level
| Field | Type | Constraints | Description |
|------|----|----|-----------|
| nick | VARCHAR(128) |  | Domain |
| identity | VARCHAR(128) |  | Domain |
| trust | INTEGER |  | Full path of the file |


### TABLE Files
links names on the file system and their hash value

| Field | Type | Constraints | Description |
|------|----|----|-----------|
| domain | VARCHAR(8192) | NOT NULL | Domain |
| thread | INTEGER | NOT NULL | Thread id |
| name | VARCHAR(8192) |  | Name of the file |
| hash | CHAR(64) | NOT NULL | Hash of the file |
| id | CHAR(64) | NOT NULL | Snowflake Id |
| modtime | INTEGER| NOT NULL | Last modification time of the file |




### TABLE Merkle
Store the 


### Invite
The invite is the way to access a domain. The invite contains one or more exchange credentials and the administrators of the group (their public key). It is encrypted with the public key of the receiver.  

| Field | Type | Size (bits)| Content |
|------|----|----|-----------|
| version | uint | 16 | version of the file format, 1.0 at the moment|
| admin | byte[32] | 32| public key of the admin that created the invite|
| config | byte[n] | variable| transport configuration in json format|

### Users


## Remote layout
The remote storage in a single folder named after the domain and contains the following files.
In the below description:9,223,372,036,854,775,8
- x is a snowflake id
- n is a numeric split id. I

All files have a version id, which is 1.0

```
📦public.safepool/main
┣📜.keys
┃ ┣1541815603606036480
┃ ┗1629405603606036480
┗📜.safepool
```


### .keys
The _.keys_ folder contain a subfolder for each thread in the domain.
Each subfolder contains encryption files for each thread


The subfolder _public_ contains the key for all the users of the domain. When 
Key files are stored under the _.key_ folder.
Each key has 


| Field | Type | Size (bits)| Content |
|------|----|----|-----------|
| version | uint | 16 | version of the file format, 1.0 at the moment|
| users | User[] | variable| list of users|

and each user consists of 

| Field | Type | Size (bits)| Content |
|------|----|----|-----------|
| public | []byte | 128 | ed25519 public key|
| flags | uint | 16 | reserved must be 0|
| aes | string | variable | symmetric encryption key used 
| name | string | variable | first name of the user|
| name2 | string | variable | second name of the user (used in case of multiple users with the same name)|


### C.x 
A change file contains an update on a file. It is made of

| Field | Type | Size (bits)| Content |
|------|----|----|-----------|
| version | uint | 16 | version of the file format, 1.0 at the moment|
| headerSize | uint | 32 | size of the header&#x00B9;, i.e. all the fields except|
| names | string[] | variable | list of names in local |
| origin | string | variable | list of names in exchanger |
| xorHash | byte[] | 256 | xor hash of all parts hashes |
| hashes | byte[][] | variable (x256) | hashes for each part of content&#x00B2; |
| message | string | variable | optional markdown message for other users before they receive the change |
| changes | Change[] | variable | changes against the origin file |
| data | byte[] | variable | the actual data |

Each Change is made in fact of
| Field | Type | Size (bits)| Content |
|------|----|----|-----------|
| type | uint | 16 | type of change: create, replace, delete, insert|
| from | uint | 32 | size of the header&#x00B9;, i.e. all the fields except|
| from | uint | 32 | size of the header&#x00B9;, i.e. all the fields except|

&#x00B9; All the fields before data are the file header

&#x00B2; Parts are built with a Hashsplit algorithm



### A.x
Action file. It defines actions each user can request to the other users. This includes:
- Truncate: delete oldest change files. This usually requires merge of oldest files with latest changes

### Sign and encryption

| File | Signed | Encrypted |
|------|--------|-----------|
| Group | &#10004;  | |
| C.x | &#10004; | &#10004; |
| K.x | &#10004; | |
| N.n | &#10004; | &#10004; |
| N.n | &#10004; | |
| A.x | &#10004; | &#10004; |

Signing is implemented with a ed25519 signing where the public/private keys are the identity of each user. 
On the file system, both the signer public key and the signature are added after the content in binary form

| Field | Type | Size (bits)| Content |
|------|----|----|-----------|
| public | uint | 256 | ed|
| hash | uint | 256 | hash value|


Encryption is implemented with AES256. 