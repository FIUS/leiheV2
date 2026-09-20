{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  # Hier kommen alle Pakete rein, die du für das Projekt brauchst
  buildInputs = with pkgs; [
    go
    sqlc
    postgresql
  ];
}
