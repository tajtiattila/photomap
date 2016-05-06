// When the window has finished loading create our google map below
google.maps.event.addDomListener(window, 'load', init);
function init() {
  // Basic options for a simple Google Map
  // For more options see: https://developers.google.com/maps/documentation/javascript/reference#MapOptions
  var mapOptions = {
      center: new google.maps.LatLng(46.0352637,18.3616772), // Baranya
      zoom: 8,

      scaleControl: true,

      // How you would like to style the map. 
      // This is where you would paste any style found on Snazzy Maps.
      styles: [{"featureType":"landscape","stylers":[{"saturation":-100},{"lightness":65},{"visibility":"on"}]},{"featureType":"poi","stylers":[{"saturation":-100},{"lightness":51},{"visibility":"simplified"}]},{"featureType":"road.highway","stylers":[{"saturation":-100},{"visibility":"simplified"}]},{"featureType":"road.arterial","stylers":[{"saturation":-100},{"lightness":30},{"visibility":"on"}]},{"featureType":"road.local","stylers":[{"saturation":-100},{"lightness":40},{"visibility":"on"}]},{"featureType":"transit","stylers":[{"saturation":-100},{"visibility":"simplified"}]},{"featureType":"administrative.province","stylers":[{"visibility":"off"}]},{"featureType":"water","elementType":"labels","stylers":[{"visibility":"on"},{"lightness":-25},{"saturation":-100}]},{"featureType":"water","elementType":"geometry","stylers":[{"hue":"#ffff00"},{"lightness":-25},{"saturation":-97}]}]
  };

  // Get the HTML DOM element that will contain your map 
  // We are using a div with id="map" seen below in the <body>
  var mapElement = document.getElementById('map');

  // Create the Google Map using our element and options defined above
  var map = new google.maps.Map(mapElement, mapOptions);

  var photoOverlay = new google.maps.ImageMapType({
    getTileUrl: function(coord, zoom) {
      return ['/tiles/tile.png?',
          'zoom=', zoom, '&x=', coord.x, '&y=', coord.y].join('');
    },
    tileSize: google.maps.Size(256, 256)
  });
  map.overlayMapTypes.push(photoOverlay);

  // coord map for debugging
  function CoordMapType(tileSize) {
    this.tileSize = tileSize;
  }

  CoordMapType.prototype.getTile = function(coord, zoom, ownerDocument) {
    var div = ownerDocument.createElement('div');
    div.innerHTML = coord;
    div.style.width = this.tileSize.width + 'px';
    div.style.height = this.tileSize.height + 'px';
    div.style.fontSize = '10';
    div.style.borderStyle = 'dashed';
    div.style.borderWidth = '1px';
    div.style.borderColor = '#050';
    return div;
  };

  /* debug map
  map.overlayMapTypes.insertAt(
      0, new CoordMapType(new google.maps.Size(256, 256)));
  */

  // Load points
  getJSON("photos.json", function(photos) {
    var pts = [];
    for (var i = 0; i < photos.length; i++) {
      var p = photos[i];
      pts.push(new google.maps.LatLng(p.lat, p.lng));
    }
    var heatmap = new google.maps.visualization.HeatmapLayer({
      data: pts,
      map: map,
      gradient: [
        'rgba(255, 128, 0, 0)',
        'rgba(255, 128, 0, 1)',
        'rgba(255, 112, 0, 1)',
        'rgba(255, 96, 0, 1)',
        'rgba(255, 80, 0, 1)',
        'rgba(255, 64, 0, 1)',
        'rgba(255, 48, 0, 1)',
        'rgba(255, 32, 0, 1)',
        'rgba(255, 16, 0, 1)',
        'rgba(255, 12, 0, 1)',
        'rgba(255, 8, 0, 1)',
        'rgba(255, 4, 0, 1)',
        'rgba(255, 0, 0, 1)'
      ]
    });
  });

  // position/zoom to url
  map.addListener('zoom_changed', function() {
    console.log(["map zoom:", map.getZoom()].join(' '));
  });
}

function getJSON(url, success) {
    var xhr = new XMLHttpRequest();
    xhr.onreadystatechange = function() {
      if (xhr.readyState == 4) {
        if (xhr.status == 200) {
          success(JSON.parse(xhr.responseText));
        } else {
          console.error(xhr.statusText);
        }
      }
    };
    xhr.open("GET", url, true);
    xhr.send();
}
