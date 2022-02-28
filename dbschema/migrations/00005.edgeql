CREATE MIGRATION m1db2yqosnay526oifb76hwsgn2qpucgpwnlsu3cwy4kc3gq32vyha
    ONTO m1dw3fdsra3d5exzyickmwbruicd3yty2gs5u3msnn6zvr3ewjyh2q
{
  ALTER TYPE default::User {
      ALTER PROPERTY editor_config {
          SET default := (std::to_json('{"autoComplete":true,"autoPairDelimiters":true,"fontFamily":"lucida","fontSize":12,"lineHeight":"normal","mode":"default","overallTheme":"","pdfViewer":"pdfjs","syntaxValidation":false,"spellCheckLanguage":"en","theme":"textmate"}'));
      };
  };
};
