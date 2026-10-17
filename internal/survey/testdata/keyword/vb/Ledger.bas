Attribute VB_Name = "Ledger"
' A VB6 module with no Option Explicit.
Private Declare Function GetTickCount Lib "kernel32" () As Long

Public Function Total(ByVal n As Integer) As Currency
    On Error Resume Next
    Total = Fee(n) * 2
End Function

Private Function Fee(n As Integer) As Currency
    Fee = CCur(n) ' Rem not a call: Total()
End Function

Public Sub Save()
    Open "ledger.dat" For Output As #1
    Print #1, Total(3)
    Close #1
End Sub
