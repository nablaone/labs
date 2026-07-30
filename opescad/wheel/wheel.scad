$fn = 200; // gładkie krzywizny

// Parametry koła
r_toczenia = 63.5;          // promień toczenia
szerokosc_biezni = 18;      // szerokość bieżni
kat_stozka = 2.5;           // kąt bieżni
promien_luku_biezni = 8;    // promień łuku przejściowego
wysokosc_obreczy = 6;       // wysokość obrzeża
grubosc_obreczy = 5;        // szerokość obrzeża
kat_obreczy = 15;           // kąt obrzeża
promien_zaokraglenia = 2;   // zaokrąglenie czubka obrzeża
spadek_wewnetrzny = 5;      // spadek do środka
promien_osi = 10;           // promień otworu na oś (średnica 20 mm)
glebokosc_wglebienia = 10;  // głębokość wgłębienia wewnętrznego
szerokosc_wglebienia = 30;  // szerokość wgłębienia od osi

module profil_kola() {
    n = 20;
    m = 20;
    stozenie_biezni = tan(kat_stozka * PI/180);
    stozenie_obreczy = tan(kat_obreczy * PI/180);

    bieznia_x = r_toczenia - szerokosc_biezni;
    bieznia_y = -stozenie_biezni * szerokosc_biezni;

    // Start zdefiniowany jako tablica
    points = [
        [r_toczenia, 0],                        // początek bieżni
        [bieznia_x, bieznia_y]                  // koniec bieżni
    ];

    // Łuk przejściowy bieżnia->obrzeże
    for (i = [0 : n]) {
        angle = 90 * i / n;
        x = bieznia_x - promien_luku_biezni * sin(angle * PI / 180);
        y = bieznia_y - promien_luku_biezni + promien_luku_biezni * cos(angle * PI / 180);
        points = concat(points, [[x, y]]);
    }

    // Obrzeże pod kątem
    x_top = r_toczenia + grubosc_obreczy;
    y_top = points[len(points)-1][1]; // ostatni y
    x_bottom = x_top - stozenie_obreczy * wysokosc_obreczy;
    y_bottom = y_top + wysokosc_obreczy;

    points = concat(points, [[x_top, y_top]]);
    points = concat(points, [[x_bottom, y_bottom - promien_zaokraglenia]]);

    // Zaokrąglenie czubka obrzeża (łuk 90°)
    for (i = [0 : m]) {
        angle = 90 * i / m;
        x = x_bottom - promien_zaokraglenia * cos(angle * PI / 180);
        y2 = (y_bottom - promien_zaokraglenia) + promien_zaokraglenia * sin(angle * PI / 180);
        points = concat(points, [[x, y2]]);
    }



    polygon(points);
}

difference() {
    rotate_extrude(convexity = 10)
        profil_kola();
    
    // Otwór centralny
    cylinder(h = 100, r = promien_osi, center = true);
}
