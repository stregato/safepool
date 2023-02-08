# Introduction
The transport folder contains the drivers to write and read from storage services, including _SFTP_ and _S3_.

# Key Principles
- Simple interface
- Optimized for cloud storage services (e.g. use of multi-uploader)

# Interfaces

    interface Exchanger {
        Read(name, dest) error
        Write(name, source) error
        Stat(name) stat
        Delete(name) error
        Close()
    }

    func NewStorer(config) Exchanger



