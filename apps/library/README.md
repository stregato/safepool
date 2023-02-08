# Library API
The library API allows to share documents with other nodes in the decentralized network. 
Like for usual data repository (e.g. Sharepoint) 

## The API
```
    func Get(p *pool.Pool, channel string) Library

    func (l *Library) List(folder string) ([]Document, error)   
    func (l *Library) Send(localPath string, name string, tags ...string) (pool.Head, error)
    func (l *Library) Receive(id uint64, localPath string, tags ...string) (pool.Head, error)
    func (l *Library) Delete(id uint64) error
    func (l *Library) Save(id uint64, dest string) error


    func (l *Library) GetLocalPath(name string) (string, bool) 
    func (l *Library) GetLocalDocument(name string) (Document, bool) 
```

## The life cycle
Let's consider a simple pool with only two nodes, Alice and Bob. 
Alice works on a document _Hello.doc_ with Bob. 

|#| Action | Alice Disk | Alice State | Bob Disk | Bob State | Pool |  
|--|-|-----|----|----|----|----|
|1| Alice creates a file | hello.doc | - | - | - | - |
|2| Alice sends to the pool | hello.doc | Sync | - | Update | hello.doc |
|3| Bob receives from the pool | hello.doc | Sync | - | Sync | hello.doc |
|4| Alice deletes the file | - | Deleted | - | Sync | hello.doc |
|5| Bop edit the file | - | Deleted | - | Modified | hello.doc |
|6| Bop sends to the pool | - | Update | - | Sync | hello.doc |
|7| Alice receives the file | hello.doc | Sync | - | Sync | hello.doc |
|8| Alice edit the file | hello.doc | Modified | - | Update | hello.doc |

The state represents the condition of the local file compared with the file with the same name on the pool
State is computed by comparing 

| State | Condition (local) | Condition (remote) |
|-|-|-|
|Sync| Same mod times | Same Hash
|Modified| Newer on disk  |
|Updated| Older on disk | Hash is contained |
|Conflict| | Hash mismatch |
|Deleted| File does not exist |






