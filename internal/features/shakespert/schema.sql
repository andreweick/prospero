-- Schema for Shakespeare database (based on shakespert.db structure)

CREATE TABLE Works (
  WorkID varchar(50) PRIMARY KEY,
  Title varchar(255),
  LongTitle varchar(255),
  ShortTitle varchar(255),
  Date integer,
  GenreType varchar(255),
  Notes blob,
  Source varchar(255),
  TotalWords integer,
  TotalParagraphs integer
);

CREATE TABLE Genres (
  GenreType varchar(255) PRIMARY KEY,
  GenreName varchar(255)
);

CREATE TABLE Characters (
  CharID varchar(50) PRIMARY KEY,
  CharName varchar(255),
  Abbrev varchar(255),
  Works varchar(255),
  Description varchar(255),
  SpeechCount integer
);

CREATE TABLE Paragraphs (
  WorkID varchar(255),
  ParagraphID integer PRIMARY KEY,
  ParagraphNum integer,
  CharID varchar(255),
  PlainText text,
  PhoneticText text,
  StemText text,
  ParagraphType char(1),
  Section integer,
  Chapter integer,
  CharCount integer,
  WordCount integer
);

CREATE TABLE Chapters (
  WorkID varchar(255),
  ChapterID integer PRIMARY KEY,
  Section integer,
  Chapter integer,
  Description varchar(255)
);