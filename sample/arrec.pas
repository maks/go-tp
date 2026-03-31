program arrec;

type
  Point = record
    x: integer;
    y: integer;
  end;

var
  pts: array[1..3] of integer;
  p: Point;
  i: integer;
  sum: integer;

begin
  { fill array }
  pts[1] := 10;
  pts[2] := 20;
  pts[3] := 30;

  { sum array elements }
  sum := 0;
  for i := 1 to 3 do
    sum := sum + pts[i];
  writeln(sum);

  { use a record }
  p.x := 7;
  p.y := 3;
  writeln(p.x + p.y);
end.
