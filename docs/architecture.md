- **The shape** — We make http request, it gets to this server, here its handled an redirected to the proper function. There then we do some validation then we acceess the LinkStore and store the link. then we return a http response.
- **The data** — Everything is stored in memroy in the LinkStore. When we reset, everything is lost.
- **Three decisions** — 
1. For persisting data I didnt initally want to do it in memory and wanted to use a cloud database. I tried to do something with the sqlx package but indeed it seemed to me as complex as managing the state in memory. Since the complexity seemed the same I decided to keep going with the in memory variant.
2. Making a REST API just with GO's basic packages seemed a bit hard so I wanted to search for libraries that would make this project feel more like ASP.NET or Spring Boot. I abandond this idea due to lack of time.
3. x
- **What is not built yet** — I still have to work on the following endpoints: /{code} ,/links/{code}/stats, /links/{code}. Also more work needs to be done on validation. I also wanna move some logc out of main.go into some internal packages.