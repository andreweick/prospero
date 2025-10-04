-- name: ListWorks :many
SELECT w.WorkID, w.Title, w.LongTitle, w.Date, w.GenreType, g.GenreName, w.TotalWords, w.TotalParagraphs
FROM Works w
LEFT JOIN Genres g ON w.GenreType = g.GenreType
ORDER BY w.Title;

-- name: GetWork :one
SELECT w.WorkID, w.Title, w.LongTitle, w.ShortTitle, w.Date, w.GenreType, g.GenreName, w.Notes, w.Source, w.TotalWords, w.TotalParagraphs
FROM Works w
LEFT JOIN Genres g ON w.GenreType = g.GenreType
WHERE w.WorkID = ?;

-- name: ListGenres :many
SELECT GenreType, GenreName
FROM Genres
ORDER BY GenreName;

-- name: GetWorksByGenre :many
SELECT w.WorkID, w.Title, w.LongTitle, w.Date, w.GenreType, g.GenreName, w.TotalWords, w.TotalParagraphs
FROM Works w
LEFT JOIN Genres g ON w.GenreType = g.GenreType
WHERE w.GenreType = ?
ORDER BY w.Title;

-- name: GetWorkCharacters :many
SELECT DISTINCT c.CharID, c.CharName, c.Description, c.SpeechCount
FROM Characters c
WHERE c.Works LIKE '%' || ? || '%'
ORDER BY c.CharName;

-- name: GetWorkChapters :many
SELECT ChapterID, Section, Chapter, Description
FROM Chapters
WHERE WorkID = ?
ORDER BY Section, Chapter;