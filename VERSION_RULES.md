Rule #1: API elements may only be removed by incrementing the version of the API group.

Rule #2: API objects must be able to round-trip between API versions in a given release without information loss, with the exception of whole REST resources that do not exist in some versions.

Rule #3: An API version in a given track may not be deprecated until a new API version at least as stable is released.

Rule #4a: Other than the most recent API versions in each track, older API versions must be supported after their announced deprecation for a duration of no less than:

Rule #4a: Other than the most recent API versions in each track, older API versions must be supported after their announced deprecation for a duration of no less than:
- GA: 12 months or 3 releases (whichever is longer)
- Beta: 9 months or 3 releases (whichever is longer)
- Alpha: 0 releases

Rule #4b: The "preferred" API version and the "storage version" for a given group may not advance until after a release has been made that supports both the new version and the previous version


