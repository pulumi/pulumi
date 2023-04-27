// Because basePath is set in tsconfig, as long as tsconfig is loaded,
// this package should be found.

import { MyFavoriteType } from "myLibrary";

const greeter: MyFavoriteType = {
  greeting: "Bonjour!",
};

console.log(greeter);