search
======

This is a service which serves as a search facade for the DE and others to use. It uses the querydsl library under the covers to translate requests and provide documentation, then passes off queries to configured elasticsearch servers.

At present, the service supports searching only data.

configuration
-------------

The configuration file is a YAML file. It should have two top-level objects, `elasticsearch` and `data_info`. Each should have a subkey `base` which specifies the base URL of the respective service; the `elasticsearch` object should also have `user`, `password`, and `index`.

endpoints
---------

The service has two endpoints:

 * /data/documentation responds to GET requests with documentation of the available querydsl clauses and their arguments/types, plus the list of available sort fields
 * /data/search responds to POST requests. It expects a 'user' query parameter, and within the body of the request a querydsl-compatible JSON query inside a JSON object under the key 'query'. It optionally accepts integers in the "from" and "size" keys of the body, which specify the offset and the limit respectively. The defaults are 0 for "from" and 10 for "size". It optionally accepts a "sort" key, which should be an array of objects with "field" and "order" subfields, where "order" is either "ascending" or "descending" and the fields are within the values listed in the documentation endpoint. This endpoint automatically filters results to those the user can see, and adds a "permission" field that summarizes the requesting user's effective permission on each result.
