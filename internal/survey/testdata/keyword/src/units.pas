unit Units;
{$R-}
interface
uses SysUtils, Helpers;
function Convert(x: Double): Double;
implementation
{ a block comment
  procedure Fake; }
function Scale(x: Double): Double;
begin
  Scale := x * 2;
end;
function Convert(x: Double): Double;
begin
  Convert := Scale(x) + 1; // calls Scale
end;
end.
