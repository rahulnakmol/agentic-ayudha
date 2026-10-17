import org.json.*;
import java.nio.file.*;

public class Driver {
    public static class Bean {
        public String getName() { return "Ünïcode ı"; }
        public double getRatio() { return 1234567.891; }
        public long getCount() { return 1234567L; }
        public boolean isActive() { return true; }
        public java.math.BigDecimal getPrice() { return new java.math.BigDecimal("0.10"); }
    }
    public static void main(String[] a) throws Exception {
        String in = a.length > 1 ? new String(Files.readAllBytes(Paths.get(a[1])), "UTF-8") : "";
        switch (a[0]) {
            case "roundtrip": System.out.println(new JSONObject(in).toString(2)); break;
            case "xml": System.out.println(XML.toJSONObject(in).toString(2)); break;
            case "pointer":
                try { System.out.println(new JSONPointer(a[2]).queryFrom(new JSONObject(in))); }
                catch (JSONPointerException e) { System.out.println(e.getMessage()); System.exit(2); }
                break;
            case "bean": System.out.println(new JSONObject(new Bean()).toString(2)); break;
            case "strict":
                try { new JSONObject(in, new JSONParserConfiguration().withStrictMode(true)); System.out.println("ok"); }
                catch (JSONException e) { System.out.println(e.getMessage()); System.exit(1); }
                break;
        }
    }
}
