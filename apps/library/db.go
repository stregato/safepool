package library

import (
	"encoding/json"
	"path"
	"strings"

	"github.com/code-to-go/safe/safepool/core"
	"github.com/code-to-go/safe/safepool/sql"
)

func sqlSetDocument(pool string, base string, d File) error {
	folder, level := getFolderAndLevel(path.Dir(d.Name))
	hashChain, _ := json.Marshal(d.HashChain)

	_, err := sql.Exec("SET_LIBRARY_FILE", sql.Args{"pool": pool, "base": base, "id": d.Id, "name": d.Name,
		"authorId": d.AuthorId, "modTime": sql.EncodeTime(d.ModTime), "size": d.Size,
		"contentType": d.ContentType, "hash": sql.EncodeBase64(d.Hash), "hashChain": hashChain,
		"offset": d.Offset, "folder": folder, "level": level})
	if core.IsErr(err, "cannot set document %d on db: %v", d.Name) {
		return err
	}

	return err
}

func sqlGetDocumentByArgs(key string, args sql.Args) (File, bool, error) {
	var d File
	var hash string
	var hashChain []byte
	var modTime int64

	err := sql.QueryRow(key, args,
		&d.Name, &d.AuthorId, &modTime, &d.Id, &d.Size, &d.ContentType, &hash, &hashChain, &d.Offset)
	if err == sql.ErrNoRows {
		return File{}, false, nil
	} else if core.IsErr(err, "cannot get %s file %v from DB: %v", key, args) {
		return d, false, err
	}

	d.Hash = sql.DecodeBase64(hash)
	json.Unmarshal(hashChain, &d.HashChain)
	d.ModTime = sql.DecodeTime(modTime)

	return d, true, nil
}

func sqlGetDocumentById(pool string, base string, id uint64) (File, bool, error) {
	return sqlGetDocumentByArgs("GET_LIBRARY_FILE_BY_ID", sql.Args{"pool": pool, "base": base, "id": id})
}

// func sqlGetDocumentByName(pool string, base string, name string, authorId string) (File, bool, error) {
// 	return sqlGetDocumentByArgs("GET_DOCUMENT_BY_NAME", sql.Args{"pool": pool, "base": base, "name": name, "authorId": authorId})
// }

func getFolderAndLevel(folder string) (string, int) {
	folder = path.Clean(folder)
	folder = strings.TrimPrefix(folder, ".")
	folder = strings.Trim(folder, "/")
	level := strings.Count(folder, "/")
	return folder, level
}

func sqlGetSubfolders(pool string, base string, folder string) ([]string, error) {
	folder, level := getFolderAndLevel(folder)
	rows, err := sql.Query("GET_LIBRARY_FILES_SUBFOLDERS", sql.Args{"pool": pool, "base": base, "folder": folder + "%", "level": level + 1})
	if core.IsErr(err, "cannot query documents from db: %v") {
		return nil, err
	}
	var subfolders []string
	for rows.Next() {
		var subfolder string
		err = rows.Scan(&subfolder)
		if !core.IsErr(err, "cannot scan row in getting subfolder: %v", err) {
			subfolders = append(subfolders, subfolder)
		}
	}
	return subfolders, nil
}

//name,authorId,mode,id,size,contentType,hash,hashChain,localPath,offset

func sqlFilesInFolder(pool string, base string, folder string) ([]File, error) {
	rows, err := sql.Query("GET_LIBRARY_FILES_IN_FOLDER", sql.Args{"pool": pool, "base": base, "folder": folder})
	if core.IsErr(err, "cannot query documents from db: %v") {
		return nil, err
	}
	var documents []File
	for rows.Next() {
		var d File
		var hash string
		var hashChain []byte
		var modTime int64

		err = rows.Scan(&d.Name, &d.AuthorId, &modTime, &d.Id, &d.Size, &d.ContentType, &hash, &hashChain, &d.Offset)
		if !core.IsErr(err, "cannot scan row in Files: %v", err) {
			d.Hash = sql.DecodeBase64(hash)
			json.Unmarshal(hashChain, &d.HashChain)
			d.ModTime = sql.DecodeTime(modTime)
			documents = append(documents, d)
		}
	}
	return documents, nil
}

func sqlGetOffset(pool string, base string) int {
	var offset int
	err := sql.QueryRow("GET_LIBRARY_FILES_OFFSET", sql.Args{"pool": pool, "base": base}, &offset)
	if err == nil {
		return offset
	} else {
		return -1
	}
}

func sqlSetLocal(pool string, base string, l Local) error {
	folder, _ := getFolderAndLevel(path.Dir(l.Name))
	hashChain, err := json.Marshal(l.HashChain)
	if core.IsErr(err, "cannot serialize hash chain: %v") {
		return err
	}

	_, err = sql.Exec("SET_LIBRARY_LOCAL", sql.Args{"pool": pool, "base": base, "folder": folder,
		"name": l.Name, "path": l.Path, "id": l.Id, "modTime": sql.EncodeTime(l.ModTime),
		"authorId": l.AuthorId, "size": l.Size,
		"hash": sql.EncodeBase64(l.Hash), "hashChain": hashChain})
	if core.IsErr(err, "cannot set local %d on db: %v", l.Name) {
		return err
	}
	return nil
}

func sqlGetLocal(pool, base, name string) (Local, bool, error) {
	//SELECT name,path,id,authorId,modTime,size,hash,hashChain FROM library_locals WHERE pool=:pool AND base=:base AND name=:name

	var l Local
	var hash string
	var modTime int64
	var hashChain []byte
	err := sql.QueryRow("GET_LIBRARY_LOCAL", sql.Args{"pool": pool, "base": base, "name": name},
		&l.Name, &l.Path, &l.Id, &l.AuthorId, &modTime, &l.Size, &hash, &hashChain)
	if err == sql.ErrNoRows {
		return l, false, nil
	}
	if core.IsErr(err, "cannot get local for name %s on db: %v", l.Name) {
		return l, false, err
	}
	l.ModTime = sql.DecodeTime(modTime)
	l.Hash = sql.DecodeBase64(hash)
	json.Unmarshal(hashChain, &l.HashChain)

	return l, true, nil
}

func sqlGetLocalsInFolder(pool string, base string, folder string) ([]Local, error) {
	//SELECT name,path,id,authorId,modTime,size,hash,hashChain FROM library_locals WHERE pool=:pool AND base=:base AND folder=:folder
	rows, err := sql.Query("GET_LIBRARY_LOCALS_IN_FOLDER", sql.Args{"pool": pool, "base": base, "folder": folder})
	if core.IsErr(err, "cannot query documents from db: %v") {
		return nil, err
	}
	var locals []Local
	for rows.Next() {
		var l Local
		var hash string
		var hashChain []byte
		var modTime int64

		err = rows.Scan(&l.Name, &l.Path, &l.Id, &l.AuthorId, &modTime, &l.Size, &hash, &hashChain)
		if !core.IsErr(err, "cannot scan row in Locals: %v", err) {
			l.ModTime = sql.DecodeTime(modTime)
			l.Hash = sql.DecodeBase64(hash)
			json.Unmarshal(hashChain, &l.HashChain)
			locals = append(locals, l)
		}
	}
	return locals, nil
}

func sqlGetFilesHashes(pool string, base string, name string, limit int) ([][]byte, error) {
	rows, err := sql.Query("GET_LIBRARY_FILES_HASHES", sql.Args{"pool": pool, "base": base, "name": name, "limit": limit})
	if core.IsErr(err, "cannot get file hashes from db: %v") {
		return nil, err
	}
	var hashes [][]byte
	for rows.Next() {
		var hash string
		err = rows.Scan(&hash)
		if !core.IsErr(err, "cannot scan row in file hashes: %v", err) {
			hashes = append(hashes, sql.DecodeBase64(hash))
		}
	}
	return hashes, nil
}
