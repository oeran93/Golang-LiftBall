
--
This program includes a client and a server. 
The server keeps an on-line backup of all files in a particular directory and syncs the files from the on-line
service with a local copy of the files.

Server Program

The server program takes no parameters when started.
When the program starts it makes sure there is a directory called liftballBackup.
When a connection is made to a client, the server syncs the files from the client with the files in the liftballBackup
directory. The server responds to requests for a listing of the files in the directory.
The server also responds to requests for individual files by returning the requested file to the client.
The server also responds to requests to store a new file and delete an existing file.
The server handles multiple connections at the same time.

Client Program

The client program takes a single parameter of the ip address of the server.
The client also has a simple qml UI that allows the user to request files, delete files and sync files with the server.
