# Luzifer / clean_couch

[![License: Apache 2.0](http://badge.luzifer.io/v1/badge?color=5d79b5&title=license&text=Apache%202.0)](http://www.apache.org/licenses/LICENSE-2.0)

This utility emerged from the need to delete about 20k documents from a CouchDB database with more than 600k documents. As I did not want to delete every document by hand and had no other way to delete documents by a specific filter.

## Usage

1. Create a view which filters the documents in your database with exactly this emit line you can see in this example

```javascript
function(doc) {
  if (doc.user == "usertodelete") {
    emit(doc._rev, null);
  }
}
```

2. Execute with parameters

```bash
# ./clean_couch
Usage of ./clean_couch:
      --baseurl="http://localhost:5984": BaseURL of your CouchDB instance
      --concurrency=50: How many delete requests should get processed concurrently?
      --database="": The database containing your view and the data to delete
      --view="": The view selecting the data to delete

# ./clean_couch --database=userdata --view=_design/del/_view/usertodelete
```

## Warnings

- If you set the concurrency above 1024 either `clean_couch` or even the CouchDB server might break because of a limit in open file descriptors
- If the database has many views you could overload your server because views need to get recalculated  
(My CouchDB server survived a concurrency of 100 with minimal load)
